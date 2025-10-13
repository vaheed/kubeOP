package logging

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"kubeop/internal/version"
)

const serviceName = "kubeop"

var (
	globalLogger atomic.Pointer[zap.Logger]
	globalAudit  atomic.Pointer[zap.Logger]
)

func init() {
	globalLogger.Store(zap.NewNop())
	globalAudit.Store(zap.NewNop())
}

// Metadata describes build information that should be attached to every log.
type Metadata struct {
	Version   string
	Commit    string
	ClusterID string
}

// Manager wires structured logging for the API. It keeps track of the logging
// configuration so log files can be reopened (SIGHUP) and flushed on shutdown.
type Manager struct {
	mu          sync.Mutex
	cfg         Config
	meta        Metadata
	level       zap.AtomicLevel
	auditLevel  zap.AtomicLevel
	appWriter   *lumberjack.Logger
	auditWriter *lumberjack.Logger
}

// Config is derived from environment variables.
type Config struct {
	Level       string
	Dir         string
	MaxSizeMB   int
	MaxBackups  int
	MaxAgeDays  int
	Compress    bool
	AuditEnable bool
	ClusterID   string
}

// ReadConfig extracts logging configuration from environment variables. It
// applies defaults when a variable is not provided or malformed.
func ReadConfig() Config {
	cfg := Config{
		Level:       getenv("LOG_LEVEL", "info"),
		Dir:         getenv("LOG_DIR", "/var/log/kubeop"),
		MaxSizeMB:   getenvInt("LOG_MAX_SIZE_MB", 50),
		MaxBackups:  getenvInt("LOG_MAX_BACKUPS", 7),
		MaxAgeDays:  getenvInt("LOG_MAX_AGE_DAYS", 14),
		Compress:    getenvBool("LOG_COMPRESS", true),
		AuditEnable: getenvBool("AUDIT_ENABLED", true),
		ClusterID:   strings.TrimSpace(os.Getenv("CLUSTER_ID")),
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 50
	}
	if cfg.MaxBackups < 0 {
		cfg.MaxBackups = 7
	}
	if cfg.MaxAgeDays < 0 {
		cfg.MaxAgeDays = 14
	}
	return cfg
}

// Setup initialises global application and audit loggers. The returned manager
// can reopen log files on SIGHUP and flush them on shutdown.
func Setup(meta Metadata) (*Manager, error) {
	cfg := ReadConfig()
	if strings.TrimSpace(meta.Version) == "" {
		meta.Version = version.Version
	}
	if strings.TrimSpace(meta.Commit) == "" {
		meta.Commit = version.Commit
	}
	if meta.ClusterID == "" {
		meta.ClusterID = cfg.ClusterID
	}
	m := &Manager{cfg: cfg, meta: meta}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.rebuildLocked(true); err != nil {
		return nil, err
	}
	m.logConfig()
	return m, nil
}

// L returns the global application logger.
func L() *zap.Logger {
	return globalLogger.Load()
}

// Audit returns the global audit logger.
func Audit() *zap.Logger {
	return globalAudit.Load()
}

// Reopen rebuilds the loggers, refreshing lumberjack handles. It is safe to
// call concurrently with logging operations.
func (m *Manager) Reopen() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.rebuildLocked(false); err != nil {
		fmt.Fprintf(os.Stderr, "logging: reopen failed: %v\n", err)
		return
	}
	L().Info("log files reopened", zap.String("signal", "SIGHUP"))
}

// Sync flushes both application and audit loggers.
func (m *Manager) Sync() {
	syncLogger(L())
	syncLogger(Audit())
}

func (m *Manager) logConfig() {
	L().Info("logging configured",
		zap.String("service", serviceName),
		zap.String("level", strings.ToLower(m.cfg.Level)),
		zap.String("directory", m.cfg.Dir),
		zap.Int("max_size_mb", m.cfg.MaxSizeMB),
		zap.Int("max_backups", m.cfg.MaxBackups),
		zap.Int("max_age_days", m.cfg.MaxAgeDays),
		zap.Bool("compress", m.cfg.Compress),
		zap.Bool("audit_enabled", m.cfg.AuditEnable),
	)
}

func (m *Manager) rebuildLocked(initial bool) error {
	if !initial {
		// drop previous writers so we don't leak file descriptors.
		if m.appWriter != nil {
			_ = m.appWriter.Close()
		}
		if m.auditWriter != nil {
			_ = m.auditWriter.Close()
		}
	}

	lvl := parseLevel(m.cfg.Level)
	m.level = zap.NewAtomicLevelAt(lvl)
	m.auditLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)

	encoder := newJSONEncoder()
	stdoutCore := zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), m.level)

	cores := []zapcore.Core{stdoutCore}
	var fileWarn error
	if m.cfg.Dir != "" {
		if err := os.MkdirAll(m.cfg.Dir, 0o755); err != nil {
			fileWarn = fmt.Errorf("create log dir: %w", err)
			fmt.Fprintf(os.Stderr, "logging: %v; falling back to stdout only\n", fileWarn)
		} else {
			m.appWriter = &lumberjack.Logger{
				Filename:   filepath.Join(m.cfg.Dir, "app.log"),
				MaxSize:    m.cfg.MaxSizeMB,
				MaxBackups: m.cfg.MaxBackups,
				MaxAge:     m.cfg.MaxAgeDays,
				Compress:   m.cfg.Compress,
			}
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(m.appWriter), m.level))
		}
	}

	appLogger := zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	appLogger = appLogger.With(globalFields(m.meta)...)

	auditLogger := zap.NewNop()
	if m.cfg.AuditEnable {
		auditCores := make([]zapcore.Core, 0, 2)
		if m.cfg.Dir != "" && fileWarn == nil {
			m.auditWriter = &lumberjack.Logger{
				Filename:   filepath.Join(m.cfg.Dir, "audit.log"),
				MaxSize:    m.cfg.MaxSizeMB,
				MaxBackups: m.cfg.MaxBackups,
				MaxAge:     m.cfg.MaxAgeDays,
				Compress:   m.cfg.Compress,
			}
			auditCores = append(auditCores, zapcore.NewCore(encoder, zapcore.AddSync(m.auditWriter), m.auditLevel))
		}
		if len(auditCores) == 0 {
			// When audit logging can't write to disk, fall back to stdout while warning.
			if fileWarn != nil {
				fmt.Fprintf(os.Stderr, "logging: audit disabled due to %v; using stdout fallback\n", fileWarn)
			}
			auditCores = append(auditCores, stdoutCore)
		}
		auditLogger = zap.New(zapcore.NewTee(auditCores...), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
		auditLogger = auditLogger.With(globalFields(m.meta)...)
	}

	globalLogger.Store(appLogger)
	globalAudit.Store(auditLogger)
	zap.ReplaceGlobals(appLogger)

	if fileWarn != nil && !initial {
		appLogger.Warn("log file rebuild encountered error", zap.String("error", fileWarn.Error()))
	}
	return nil
}

func parseLevel(level string) zapcore.Level {
	if lvl, err := zapcore.ParseLevel(strings.ToLower(strings.TrimSpace(level))); err == nil {
		return lvl
	}
	return zapcore.InfoLevel
}

func newJSONEncoder() zapcore.Encoder {
	encCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.UTC().Format(time.RFC3339Nano))
		},
	}
	return zapcore.NewJSONEncoder(encCfg)
}

func globalFields(meta Metadata) []zap.Field {
	fields := []zap.Field{
		zap.String("service", serviceName),
		zap.String("version", meta.Version),
		zap.String("commit", meta.Commit),
	}
	if meta.ClusterID != "" {
		fields = append(fields, zap.String("cluster_id", meta.ClusterID))
	}
	return fields
}

func syncLogger(l *zap.Logger) {
	if l == nil {
		return
	}
	if err := l.Sync(); err != nil && !isIgnorableSyncErr(err) {
		fmt.Fprintf(os.Stderr, "logging: sync failed: %v\n", err)
	}
}

func isIgnorableSyncErr(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrClosed) {
		return true
	}
	if runtime.GOOS == "windows" {
		return false
	}
	// Ignore common errors from stdout/stderr sync on Linux containers.
	if strings.Contains(err.Error(), "bad file descriptor") || strings.Contains(err.Error(), "invalid argument") {
		return true
	}
	return false
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(v)
	}
	return def
}

func getenvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return def
}
