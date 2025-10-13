package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

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
	fm.root = filepath.Clean(fm.root)
	absRoot, err := filepath.Abs(fm.root)
	if err != nil {
		return nil, fmt.Errorf("resolve logs root: %w", err)
	}
	fm.root = absRoot
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
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '-', '_', '.':
			continue
		default:
			return "", fmt.Errorf("invalid character %q in path segment", r)
		}
	}
	return cleaned, nil
}

func sanitizeSegments(parts []string) ([]string, error) {
	out := make([]string, len(parts))
	for i, part := range parts {
		clean, err := sanitizeSegment(part)
		if err != nil {
			return nil, err
		}
		out[i] = clean
	}
	return out, nil
}

func ensureWithin(base, candidate string) (string, error) {
	baseClean := filepath.Clean(base)
	candidateClean := filepath.Clean(candidate)
	if candidateClean == baseClean {
		return candidateClean, nil
	}
	prefix := baseClean + string(os.PathSeparator)
	if !strings.HasPrefix(candidateClean, prefix) {
		return "", fmt.Errorf("path %q escapes logs root %q", candidateClean, baseClean)
	}
	return candidateClean, nil
}

func (fm *FileManager) joinWithinRoot(parts ...string) (string, error) {
	if fm == nil {
		return "", fmt.Errorf("file manager not initialised")
	}
	if fm.root == "" {
		return "", fmt.Errorf("logs root not configured")
	}
	cleanParts, err := sanitizeSegments(parts)
	if err != nil {
		return "", fmt.Errorf("invalid path segment: %w", err)
	}
	joined := filepath.Join(append([]string{fm.root}, cleanParts...)...)
	return ensureWithin(fm.root, joined)
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
	projectDir, err := fm.joinWithinRoot("projects", cleanProjectID)
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	projectLogPath, err := fm.joinWithinRoot("projects", cleanProjectID, "project.log")
	if err != nil {
		return fmt.Errorf("resolve project log path: %w", err)
	}
	if err := ensureFile(projectLogPath); err != nil {
		return err
	}
	eventsLogPath, err := fm.joinWithinRoot("projects", cleanProjectID, "events.jsonl")
	if err != nil {
		return fmt.Errorf("resolve events log path: %w", err)
	}
	if err := ensureFile(eventsLogPath); err != nil {
		return err
	}
	appsDir, err := fm.joinWithinRoot("projects", cleanProjectID, "apps")
	if err != nil {
		return fmt.Errorf("resolve apps directory: %w", err)
	}
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
	appDir, err := fm.joinWithinRoot("projects", cleanProjectID, "apps", cleanAppID)
	if err != nil {
		return fmt.Errorf("resolve app directory: %w", err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("create app dir: %w", err)
	}
	appLogPath, err := fm.joinWithinRoot("projects", cleanProjectID, "apps", cleanAppID, "app.log")
	if err != nil {
		return fmt.Errorf("resolve app log path: %w", err)
	}
	if err := ensureFile(appLogPath); err != nil {
		return err
	}
	appErrPath, err := fm.joinWithinRoot("projects", cleanProjectID, "apps", cleanAppID, "app.err.log")
	if err != nil {
		return fmt.Errorf("resolve app error log path: %w", err)
	}
	if err := ensureFile(appErrPath); err != nil {
		return err
	}
	return nil
}

func (fm *FileManager) getLogger(parts ...string) (*zap.Logger, error) {
	if fm == nil {
		return zap.NewNop(), nil
	}
	if err := fm.ensureRoot(); err != nil {
		return nil, err
	}
	cleanParts, err := sanitizeSegments(parts)
	if err != nil {
		return nil, err
	}
	cleanRel := filepath.Join(cleanParts...)
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if h, ok := fm.handles[cleanRel]; ok {
		return h.logger, nil
	}
	fullPath, err := fm.joinWithinRoot(cleanParts...)
	if err != nil {
		return nil, err
	}
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
	logger, err := fm.getLogger("projects", cleanProjectID, "project.log")
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
	logger, err := fm.getLogger("projects", cleanProjectID, "events.jsonl")
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
	logger, err := fm.getLogger("projects", cleanProjectID, "apps", cleanAppID, "app.log")
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
	logger, err := fm.getLogger("projects", cleanProjectID, "apps", cleanAppID, "app.err.log")
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
