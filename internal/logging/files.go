package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type rotationConfig struct {
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

type fileHandle struct {
	logger *zap.Logger
	writer *lumberjack.Logger
}

type FileManager struct {
	root    string
	meta    Metadata
	rot     rotationConfig
	mu      sync.Mutex
	handles map[string]fileHandle
}

func NewFileManager(root string, rot rotationConfig, meta Metadata) (*FileManager, error) {
	fm := &FileManager{
		root:    strings.TrimSpace(root),
		meta:    meta,
		rot:     rot,
		handles: make(map[string]fileHandle),
	}
	if fm.root == "" {
		fm.root = "/var/log/kubeop"
	}
	if err := fm.ensureRoot(); err != nil {
		return nil, err
	}
	return fm, nil
}

func (fm *FileManager) ensureRoot() error {
	if fm.root == "" {
		return nil
	}
	if err := os.MkdirAll(fm.root, 0o755); err != nil {
		return fmt.Errorf("create logs root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(fm.root, "projects"), 0o755); err != nil {
		return fmt.Errorf("create projects log root: %w", err)
	}
	return nil
}

func sanitizeSegment(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", fmt.Errorf("empty path segment")
	}
	if strings.ContainsAny(trimmed, "/\\") {
		return "", fmt.Errorf("path separator not allowed")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("path segment cannot be %q", cleaned)
	}
	if cleaned != trimmed {
		return "", fmt.Errorf("path segment normalizes to %q", cleaned)
	}
	return cleaned, nil
}

func sanitizeRelPath(relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	if cleaned == "." {
		return "", fmt.Errorf("relative path %q resolves to current directory", relPath)
	}
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("relative path %q must not be absolute", relPath)
	}
	if strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("relative path %q escapes logs root", relPath)
	}
	return cleaned, nil
}

func (fm *FileManager) Root() string {
	if fm == nil {
		return ""
	}
	return fm.root
}

func (fm *FileManager) EnsureBase() error {
	if fm == nil {
		return nil
	}
	return fm.ensureRoot()
}

func (fm *FileManager) EnsureProject(projectID string, appIDs []string) error {
	if fm == nil {
		return nil
	}
	cleanProjectID, err := sanitizeSegment(projectID)
	if err != nil {
		return fmt.Errorf("invalid project id: %w", err)
	}
	if err := fm.ensureRoot(); err != nil {
		return err
	}
	projectDir := filepath.Join(fm.root, "projects", cleanProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	if err := ensureFile(filepath.Join(projectDir, "project.log")); err != nil {
		return err
	}
	if err := ensureFile(filepath.Join(projectDir, "events.jsonl")); err != nil {
		return err
	}
	appsDir := filepath.Join(projectDir, "apps")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return fmt.Errorf("create apps dir: %w", err)
	}
	cleanAppIDs := make([]string, 0, len(appIDs))
	for _, appID := range appIDs {
		cleanAppID, err := sanitizeSegment(appID)
		if err != nil {
			return fmt.Errorf("invalid app id %q: %w", appID, err)
		}
		cleanAppIDs = append(cleanAppIDs, cleanAppID)
	}
	sort.Strings(cleanAppIDs)
	for _, appID := range cleanAppIDs {
		if err := fm.EnsureApp(cleanProjectID, appID); err != nil {
			return err
		}
	}
	return nil
}

func (fm *FileManager) EnsureApp(projectID, appID string) error {
	if fm == nil {
		return nil
	}
	cleanProjectID, err := sanitizeSegment(projectID)
	if err != nil {
		return fmt.Errorf("invalid project id: %w", err)
	}
	cleanAppID, err := sanitizeSegment(appID)
	if err != nil {
		return fmt.Errorf("invalid app id: %w", err)
	}
	if err := fm.ensureRoot(); err != nil {
		return err
	}
	appDir := filepath.Join(fm.root, "projects", cleanProjectID, "apps", cleanAppID)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("create app dir: %w", err)
	}
	if err := ensureFile(filepath.Join(appDir, "app.log")); err != nil {
		return err
	}
	if err := ensureFile(filepath.Join(appDir, "app.err.log")); err != nil {
		return err
	}
	return nil
}

func (fm *FileManager) getLogger(relPath string) (*zap.Logger, error) {
	if fm == nil {
		return zap.NewNop(), nil
	}
	if err := fm.ensureRoot(); err != nil {
		return nil, err
	}
	cleanRel, err := sanitizeRelPath(relPath)
	if err != nil {
		return nil, err
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if h, ok := fm.handles[cleanRel]; ok {
		return h.logger, nil
	}
	fullPath := filepath.Join(fm.root, cleanRel)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return nil, fmt.Errorf("ensure log dir: %w", err)
	}
	if err := ensureFile(fullPath); err != nil {
		return nil, err
	}
	writer := &lumberjack.Logger{
		Filename:   fullPath,
		MaxSize:    fm.rot.MaxSizeMB,
		MaxBackups: fm.rot.MaxBackups,
		MaxAge:     fm.rot.MaxAgeDays,
		Compress:   fm.rot.Compress,
	}
	core := zapcore.NewCore(newJSONEncoder(), withRedactor(zapcore.AddSync(writer)), zapcore.InfoLevel)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	logger = logger.With(globalFields(fm.meta)...)
	fm.handles[cleanRel] = fileHandle{logger: logger, writer: writer}
	return logger, nil
}

func (fm *FileManager) ProjectLogger(projectID string) *zap.Logger {
	if fm == nil {
		return zap.NewNop()
	}
	cleanProjectID, err := sanitizeSegment(projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: project logger error: %v\n", err)
		return zap.NewNop()
	}
	logger, err := fm.getLogger(filepath.Join("projects", cleanProjectID, "project.log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: project logger error: %v\n", err)
		return zap.NewNop()
	}
	return logger.With(zap.String("project_id", cleanProjectID))
}

func (fm *FileManager) ProjectEventsLogger(projectID string) *zap.Logger {
	if fm == nil {
		return zap.NewNop()
	}
	cleanProjectID, err := sanitizeSegment(projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: project events logger error: %v\n", err)
		return zap.NewNop()
	}
	logger, err := fm.getLogger(filepath.Join("projects", cleanProjectID, "events.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: project events logger error: %v\n", err)
		return zap.NewNop()
	}
	return logger.With(zap.String("project_id", cleanProjectID))
}

func (fm *FileManager) AppLogger(projectID, appID string) *zap.Logger {
	if fm == nil {
		return zap.NewNop()
	}
	cleanProjectID, err := sanitizeSegment(projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: app logger error: %v\n", err)
		return zap.NewNop()
	}
	cleanAppID, err := sanitizeSegment(appID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: app logger error: %v\n", err)
		return zap.NewNop()
	}
	logger, err := fm.getLogger(filepath.Join("projects", cleanProjectID, "apps", cleanAppID, "app.log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: app logger error: %v\n", err)
		return zap.NewNop()
	}
	return logger.With(zap.String("project_id", cleanProjectID), zap.String("app_id", cleanAppID))
}

func (fm *FileManager) AppErrorLogger(projectID, appID string) *zap.Logger {
	if fm == nil {
		return zap.NewNop()
	}
	cleanProjectID, err := sanitizeSegment(projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: app error logger error: %v\n", err)
		return zap.NewNop()
	}
	cleanAppID, err := sanitizeSegment(appID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: app error logger error: %v\n", err)
		return zap.NewNop()
	}
	logger, err := fm.getLogger(filepath.Join("projects", cleanProjectID, "apps", cleanAppID, "app.err.log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: app error logger error: %v\n", err)
		return zap.NewNop()
	}
	return logger.With(zap.String("project_id", cleanProjectID), zap.String("app_id", cleanAppID))
}

func (fm *FileManager) Sync() {
	if fm == nil {
		return
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	for _, h := range fm.handles {
		syncLogger(h.logger)
	}
}

func (fm *FileManager) Close() error {
	if fm == nil {
		return nil
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	var firstErr error
	for key, h := range fm.handles {
		if err := h.writer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		syncLogger(h.logger)
		delete(fm.handles, key)
	}
	return firstErr
}

func Files() *FileManager {
	return globalFiles.Load()
}

func ProjectLogger(projectID string) *zap.Logger {
	if fm := Files(); fm != nil {
		return fm.ProjectLogger(projectID)
	}
	return zap.NewNop()
}

func ProjectEventsLogger(projectID string) *zap.Logger {
	if fm := Files(); fm != nil {
		return fm.ProjectEventsLogger(projectID)
	}
	return zap.NewNop()
}

func AppLogger(projectID, appID string) *zap.Logger {
	if fm := Files(); fm != nil {
		return fm.AppLogger(projectID, appID)
	}
	return zap.NewNop()
}

func AppErrorLogger(projectID, appID string) *zap.Logger {
	if fm := Files(); fm != nil {
		return fm.AppErrorLogger(projectID, appID)
	}
	return zap.NewNop()
}

func ensureFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("touch log file: %w", err)
	}
	return f.Close()
}
