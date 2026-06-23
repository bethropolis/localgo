package logging

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	globalSugar  *zap.SugaredLogger
)

// ANSI colour codes
const (
	colourReset  = "\033[0m"
	colourRed    = "\033[31m"
	colourYellow = "\033[33m"
	colourCyan   = "\033[36m"
	colourWhite  = "\033[37m"
	colourBold   = "\033[1m"
	colourGrey   = "\033[90m"
)

func colourLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.DebugLevel:
		enc.AppendString(colourGrey + "DBG" + colourReset)
	case zapcore.InfoLevel:
		enc.AppendString(colourCyan + "INF" + colourReset)
	case zapcore.WarnLevel:
		enc.AppendString(colourYellow + "WRN" + colourReset)
	case zapcore.ErrorLevel:
		enc.AppendString(colourRed + "ERR" + colourReset)
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		enc.AppendString(colourBold + colourRed + "FTL" + colourReset)
	default:
		enc.AppendString(l.CapitalString())
	}
}

func timeEncoder(t zapcore.TimeEncoder) zapcore.TimeEncoder {
	return t
}

// Init initialises the global zap logger.
//
//   - verbose: enable debug-level output
//   - jsonFmt: output newline-delimited JSON instead of human-readable text
//   - noColor: disable ANSI color escape sequences in log output
func Init(verbose, jsonFmt, noColor bool) *zap.SugaredLogger {
	level := zapcore.InfoLevel
	if verbose {
		level = zapcore.DebugLevel
	}

	stateDir := ""
	if xdgState := os.Getenv("XDG_STATE_HOME"); xdgState != "" {
		stateDir = filepath.Join(xdgState, "localgo")
	} else if home, err := os.UserHomeDir(); err == nil {
		stateDir = filepath.Join(home, ".local", "state", "localgo")
	}

	var fileWs zapcore.WriteSyncer
	if stateDir != "" {
		os.MkdirAll(stateDir, 0700)
		logPath := filepath.Join(stateDir, "app.log")
		if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil {
			fileWs = zapcore.Lock(f)
		}
	}
	if fileWs == nil {
		fileWs = zapcore.AddSync(os.Stderr)
	}

	var fileEnc zapcore.Encoder
	if jsonFmt {
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.TimeKey = "time"
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
		fileEnc = zapcore.NewJSONEncoder(encCfg)
	} else {
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		fileEnc = zapcore.NewConsoleEncoder(encCfg)
	}

	fileCore := zapcore.NewCore(fileEnc, fileWs, level)

	var core zapcore.Core
	if verbose {
		// Also log to stdout
		levelEnc := zapcore.LevelEncoder(colourLevelEncoder)
		if noColor {
			levelEnc = zapcore.CapitalLevelEncoder
		}
		stdoutEncCfg := zapcore.EncoderConfig{
			TimeKey:          "T",
			LevelKey:         "L",
			NameKey:          "N",
			CallerKey:        "C",
			MessageKey:       "M",
			StacktraceKey:    "S",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      levelEnc,
			EncodeTime:       zapcore.TimeEncoderOfLayout("15:04:05"),
			EncodeDuration:   zapcore.StringDurationEncoder,
			EncodeCaller:     zapcore.ShortCallerEncoder,
			ConsoleSeparator: "  ",
		}
		stdoutEnc := zapcore.NewConsoleEncoder(stdoutEncCfg)
		stdoutCore := zapcore.NewCore(stdoutEnc, zapcore.Lock(os.Stdout), level)
		core = zapcore.NewTee(fileCore, stdoutCore)
	} else {
		core = fileCore
	}

	opts := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(0)}
	if verbose {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		opts = []zap.Option{} // Minimal options for non-verbose
	}

	logger := zap.New(core, opts...)

	globalLogger = logger
	globalSugar = logger.Sugar()
	zap.ReplaceGlobals(logger)

	return globalSugar
}

// NewQuiet returns a no-op logger that discards all output.
func NewQuiet() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// Global returns the global sugared logger, or a no-op if Init has not been called.
func Global() *zap.SugaredLogger {
	if globalSugar != nil {
		return globalSugar
	}
	return zap.NewNop().Sugar()
}
