package logging

import (
	"os"

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
func Init(verbose, jsonFmt bool) *zap.SugaredLogger {
	level := zapcore.InfoLevel
	if verbose {
		level = zapcore.DebugLevel
	}

	var core zapcore.Core

	if jsonFmt {
		// JSON encoder – suitable for log aggregation pipelines
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.TimeKey = "time"
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

		enc := zapcore.NewJSONEncoder(encCfg)
		ws := zapcore.Lock(os.Stdout)
		core = zapcore.NewCore(enc, ws, level)
	} else {
		// Coloured console encoder
		encCfg := zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    colourLevelEncoder,
			EncodeTime:     zapcore.TimeEncoderOfLayout("15:04:05"),
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			ConsoleSeparator: "  ",
		}

		if !verbose {
			encCfg.CallerKey = "" // Disable caller in non-verbose mode
		}

		enc := zapcore.NewConsoleEncoder(encCfg)
		ws := zapcore.Lock(os.Stdout)
		core = zapcore.NewCore(enc, ws, level)
	}

	opts := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(0)}
	if verbose {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		opts = []zap.Option{zap.AddCallerSkip(0)} // Minimal options for non-verbose
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
