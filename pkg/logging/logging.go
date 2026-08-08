// Package logging provides LocalGo's structured logging wrapper around
// log/slog. It exposes the same printf-style surface as the previous zap
// sugared logger so call sites only depend on this package.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var (
	globalLogger *Logger
	globalSugar  *Logger
)

// ANSI colour codes
const (
	colourReset  = "\033[0m"
	colourRed    = "\033[31m"
	colourYellow = "\033[33m"
	colourCyan   = "\033[36m"
	colourGrey   = "\033[90m"
)

// Logger wraps *slog.Logger and mirrors the zap SugaredLogger method surface.
type Logger struct {
	l *slog.Logger
}

// Init initialises the global slog logger.
//
//   - verbose: enable debug-level output and also log to stdout
//   - jsonFmt: output newline-delimited JSON instead of human-readable text
//   - noColor: disable ANSI color escape sequences in log output
func Init(verbose, jsonFmt, noColor bool) *Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	stateDir := ""
	if xdgState := os.Getenv("XDG_STATE_HOME"); xdgState != "" {
		stateDir = filepath.Join(xdgState, "localgo")
	} else if home, err := os.UserHomeDir(); err == nil {
		stateDir = filepath.Join(home, ".local", "state", "localgo")
	}

	var fileWs io.Writer
	if stateDir != "" {
		os.MkdirAll(stateDir, 0700)
		logPath := filepath.Join(stateDir, "app.log")
		if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil {
			fileWs = f
		}
	}
	if fileWs == nil {
		fileWs = os.Stderr
	}

	opts := &slog.HandlerOptions{Level: level}
	if jsonFmt {
		opts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				if l, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(strings.ToLower(l.String()))
				}
			}
			return a
		}
		fileHandler := slog.NewJSONHandler(fileWs, opts)
		if verbose {
			consoleHandler := newConsoleHandler(level, noColor)
			return setGlobal(slog.New(multiHandler{handlers: []slog.Handler{fileHandler, consoleHandler}}))
		}
		return setGlobal(slog.New(fileHandler))
	}

	fileHandler := slog.NewTextHandler(fileWs, opts)
	if verbose {
		consoleHandler := newConsoleHandler(level, noColor)
		return setGlobal(slog.New(multiHandler{handlers: []slog.Handler{fileHandler, consoleHandler}}))
	}
	return setGlobal(slog.New(fileHandler))
}

func setGlobal(l *slog.Logger) *Logger {
	globalLogger = &Logger{l: l}
	globalSugar = globalLogger
	return globalSugar
}

// NewQuiet returns a no-op logger that discards all output.
func NewQuiet() *Logger {
	return &Logger{l: slog.New(slog.DiscardHandler)}
}

// Global returns the global logger, or a no-op if Init has not been called.
func Global() *Logger {
	if globalSugar != nil {
		return globalSugar
	}
	return NewQuiet()
}

func (g *Logger) Infof(format string, a ...any)  { g.l.Info(fmt.Sprintf(format, a...)) }
func (g *Logger) Warnf(format string, a ...any)  { g.l.Warn(fmt.Sprintf(format, a...)) }
func (g *Logger) Errorf(format string, a ...any) { g.l.Error(fmt.Sprintf(format, a...)) }
func (g *Logger) Debugf(format string, a ...any) { g.l.Debug(fmt.Sprintf(format, a...)) }

func (g *Logger) Info(m string)  { g.l.Info(m) }
func (g *Logger) Warn(m string)  { g.l.Warn(m) }
func (g *Logger) Error(m string) { g.l.Error(m) }
func (g *Logger) Debug(m string) { g.l.Debug(m) }

func (g *Logger) Infow(m string, kv ...any)  { g.l.Info(m, kv...) }
func (g *Logger) Warnw(m string, kv ...any)  { g.l.Warn(m, kv...) }
func (g *Logger) Errorw(m string, kv ...any) { g.l.Error(m, kv...) }
func (g *Logger) Debugw(m string, kv ...any) { g.l.Debug(m, kv...) }

// multiHandler fans records out to several slog handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (m multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return multiHandler{handlers: hs}
}

func (m multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithGroup(name)
	}
	return multiHandler{handlers: hs}
}

// consoleHandler renders human-readable lines to stdout, e.g.
//
//	15:04:05  INF  server started
type consoleHandler struct {
	level   slog.Level
	noColor bool
	w       io.Writer
}

func newConsoleHandler(level slog.Level, noColor bool) *consoleHandler {
	return &consoleHandler{level: level, noColor: noColor, w: os.Stdout}
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	buf := make([]byte, 0, 128)
	buf = r.Time.AppendFormat(buf, "15:04:05")
	buf = append(buf, ' ', ' ')

	levelStr := r.Level.String()
	if !h.noColor {
		levelStr = colourLevel(r.Level) + levelStr + colourReset
	}
	buf = append(buf, levelStr...)
	buf = append(buf, ' ', ' ')
	buf = append(buf, r.Message...)

	if r.NumAttrs() > 0 {
		buf = append(buf, ' ')
		r.Attrs(func(a slog.Attr) bool {
			buf = append(buf, a.Key...)
			buf = append(buf, '=', '\'')
			buf = append(buf, fmt.Sprint(a.Value.Any())...)
			buf = append(buf, '\'', ' ')
			return true
		})
	}

	buf = append(buf, '\n')
	_, err := h.w.Write(buf)
	return err
}

func (h *consoleHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *consoleHandler) WithGroup(_ string) slog.Handler {
	return h
}

func colourLevel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return colourRed
	case l >= slog.LevelWarn:
		return colourYellow
	case l >= slog.LevelInfo:
		return colourCyan
	default:
		return colourGrey
	}
}
