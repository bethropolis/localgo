package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	globalSugar *zap.SugaredLogger
)

func Init(verbose bool) *zap.SugaredLogger {
	var config zap.Config
	if verbose {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}

	config.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build(zap.AddCaller())
	if err != nil {
		os.Stderr.WriteString("Failed to initialize logger: " + err.Error() + "\n")
		os.Exit(1)
	}

	globalLogger = logger
	globalSugar = logger.Sugar()
	zap.ReplaceGlobals(logger)

	return globalSugar
}

func NewQuiet() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

func Global() *zap.SugaredLogger {
	if globalSugar != nil {
		return globalSugar
	}
	return zap.NewNop().Sugar()
}
