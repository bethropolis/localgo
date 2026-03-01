# ⚡ Zap — Complete Guide to Blazing-Fast Go Logging

> **Package:** `go.uber.org/zap` · **Version:** v1.27.1 · **License:** MIT  
> **Docs:** https://pkg.go.dev/go.uber.org/zap

Zap is Uber's production-grade logging library for Go. It provides structured, leveled logging with a reflection-free, zero-allocation JSON encoder — making it 4–10× faster than most other logging packages and faster than the standard library itself.

---

## Table of Contents

1. [Installation](#installation)
2. [Core Concepts](#core-concepts)
3. [Choosing a Logger](#choosing-a-logger)
4. [Logger — Strongly Typed](#logger--strongly-typed)
5. [SugaredLogger — Loosely Typed](#sugaredlogger--loosely-typed)
6. [Log Levels](#log-levels)
7. [Presets](#presets)
8. [Custom Configuration](#custom-configuration)
9. [Structured Fields](#structured-fields)
10. [Contextual Logging with `With`](#contextual-logging-with-with)
11. [Named Loggers](#named-loggers)
12. [AtomicLevel — Runtime Level Changes](#atomiclevel--runtime-level-changes)
13. [Options](#options)
14. [Advanced Configuration (zapcore)](#advanced-configuration-zapcore)
15. [Global Logger](#global-logger)
16. [Standard Library Integration](#standard-library-integration)
17. [Custom Objects and Arrays](#custom-objects-and-arrays)
18. [Writing to Files](#writing-to-files)
19. [Performance Tips](#performance-tips)
20. [Performance Benchmarks](#performance-benchmarks)

---

## Installation

```bash
go get -u go.uber.org/zap
```

Zap supports the **two most recent minor versions of Go**. For the most up-to-date compatibility matrix, see the [module page](https://pkg.go.dev/go.uber.org/zap).

---

## Core Concepts

Zap is built around two distinct loggers that trade ergonomics for performance:

| | `Logger` | `SugaredLogger` |
|---|---|---|
| API style | Strongly typed `Field` values | Loosely typed key-value pairs or `printf`-style |
| Speed | ⚡ Fastest — zero allocations | ⚡ Fast — ~10 allocs per call |
| Use when | Hot paths, production-critical code | General application code |
| Switching | `logger.Sugar()` → SugaredLogger | `sugar.Desugar()` → Logger |

Both loggers are **safe for concurrent use** and produce structured output (JSON by default in production, colored console in development).

---

## Choosing a Logger

The choice doesn't have to be all-or-nothing. Converting between the two is cheap:

```go
package main

import "go.uber.org/zap"

func main() {
    // Start with the base Logger
    logger, _ := zap.NewProduction()
    defer logger.Sync() // flush any buffered log entries

    // Get a SugaredLogger for convenience
    sugar := logger.Sugar()

    // Convert back to the base Logger when you need max performance
    plain := sugar.Desugar()

    _ = plain
}
```

---

## Logger — Strongly Typed

The `Logger` is the fastest option. Every field is explicitly typed using the `zap.Field` constructors, which eliminates reflection and keeps allocations to zero.

```go
package main

import (
    "time"
    "go.uber.org/zap"
)

func main() {
    logger, err := zap.NewProduction()
    if err != nil {
        panic(err)
    }
    defer logger.Sync()

    logger.Info("server started",
        zap.String("host", "localhost"),
        zap.Int("port", 8080),
        zap.Duration("startup_time", 42*time.Millisecond),
    )

    logger.Warn("high memory usage",
        zap.Float64("used_gb", 7.8),
        zap.Float64("total_gb", 8.0),
    )

    logger.Error("failed to connect to database",
        zap.String("dsn", "postgres://..."),
        zap.Error(err),
    )
}
```

Output (JSON):
```json
{"level":"info","ts":1735000000.123,"caller":"main.go:15","msg":"server started","host":"localhost","port":8080,"startup_time":0.042}
```

### Logger Methods

| Method | Behavior |
|---|---|
| `Debug(msg, fields...)` | Verbose, typically disabled in production |
| `Info(msg, fields...)` | Default informational level |
| `Warn(msg, fields...)` | Important but non-critical |
| `Error(msg, fields...)` | High-priority error — includes stacktrace in production |
| `DPanic(msg, fields...)` | Panics in development mode; logs normally in production |
| `Panic(msg, fields...)` | Logs then calls `panic()` |
| `Fatal(msg, fields...)` | Logs then calls `os.Exit(1)` |
| `Log(level, msg, fields...)` | Log at a dynamic level |

---

## SugaredLogger — Loosely Typed

The `SugaredLogger` is still 4–10× faster than most logging packages, with a more familiar API. It supports three calling conventions per level:

```go
sugar := logger.Sugar()
defer sugar.Sync()

// w-suffix: structured key-value pairs (recommended for structured logging)
sugar.Infow("user logged in",
    "user_id", 42,
    "ip",      "192.168.1.1",
    "method",  "oauth2",
)

// f-suffix: printf-style formatting
sugar.Infof("processing request %d of %d", current, total)

// ln-suffix: space-separated values (like fmt.Println)
sugar.Infoln("starting background worker")

// No suffix: concatenates all args (like fmt.Print)
sugar.Info("simple message")
```

### SugaredLogger Methods

Each level has four variants — shown here for `Info`:

| Method | Style |
|---|---|
| `Infow(msg, keysAndValues...)` | Structured key-value pairs |
| `Infof(template, args...)` | Printf-style |
| `Infoln(args...)` | Space-separated (fmt.Println style) |
| `Info(args...)` | Concatenated (fmt.Print style) |

The same pattern applies to `Debug`, `Warn`, `Error`, `DPanic`, `Panic`, and `Fatal`.

---

## Log Levels

```go
import "go.uber.org/zap"

// Available levels (in order of severity):
zap.DebugLevel   // -1
zap.InfoLevel    //  0  ← default minimum in production
zap.WarnLevel    //  1
zap.ErrorLevel   //  2
zap.DPanicLevel  //  3
zap.PanicLevel   //  4
zap.FatalLevel   //  5
```

Logs at levels below the configured minimum are completely **dropped at zero cost** — the level check happens before any field serialization.

---

## Presets

Zap ships three opinionated constructors for common setups:

### NewProduction

```go
logger, err := zap.NewProduction()
```

- Output: **JSON** to stderr
- Minimum level: **Info**
- Caller info: enabled
- Stacktraces: on `Error` and above
- Sampling: enabled (100 initial, then every 100th duplicate per second)

### NewDevelopment

```go
logger, err := zap.NewDevelopment()
```

- Output: **human-readable console** to stderr
- Minimum level: **Debug**
- Caller info: enabled
- Stacktraces: on `Warn` and above
- `DPanic` actually panics
- Sampling: disabled

### NewExample

```go
logger := zap.NewExample()
```

- Output: **JSON** to stdout
- Minimum level: **Debug**
- No timestamps (useful for tests and examples)

### Must — Panic on error

```go
// Convenience wrapper — panics instead of returning an error
logger := zap.Must(zap.NewProduction())
defer logger.Sync()
```

---

## Custom Configuration

The `zap.Config` struct is the recommended way to customize a logger. It supports JSON/YAML unmarshaling, making it easy to configure via config files.

```go
package main

import (
    "encoding/json"
    "go.uber.org/zap"
)

func main() {
    // Build from a JSON config string
    rawJSON := []byte(`{
        "level":       "debug",
        "encoding":    "json",
        "outputPaths": ["stdout", "/var/log/app.log"],
        "errorOutputPaths": ["stderr"],
        "initialFields": {"service": "my-api", "version": "1.0.0"},
        "encoderConfig": {
            "messageKey":  "msg",
            "levelKey":    "level",
            "timeKey":     "ts",
            "callerKey":   "caller",
            "levelEncoder": "lowercase",
            "timeEncoder":  "iso8601",
            "callerEncoder": "short"
        }
    }`)

    var cfg zap.Config
    if err := json.Unmarshal(rawJSON, &cfg); err != nil {
        panic(err)
    }

    logger := zap.Must(cfg.Build())
    defer logger.Sync()

    logger.Info("custom logger ready")
}
```

### Config Fields

| Field | Type | Description |
|---|---|---|
| `Level` | `AtomicLevel` | Minimum log level (dynamically changeable) |
| `Development` | `bool` | Enables development mode |
| `DisableCaller` | `bool` | Omit `caller` field from logs |
| `DisableStacktrace` | `bool` | Never capture stack traces |
| `Sampling` | `*SamplingConfig` | Rate-limiting for high-frequency logs |
| `Encoding` | `string` | `"json"` or `"console"` (or custom registered encoder) |
| `EncoderConfig` | `zapcore.EncoderConfig` | Control key names, time format, level format, etc. |
| `OutputPaths` | `[]string` | Write destinations: `"stdout"`, `"stderr"`, or file paths |
| `ErrorOutputPaths` | `[]string` | Where to write internal zap errors |
| `InitialFields` | `map[string]any` | Fields added to every log entry |

### Starting from a Preset Config

```go
// Customize production config without starting from scratch
cfg := zap.NewProductionConfig()
cfg.Level.SetLevel(zap.DebugLevel)
cfg.EncoderConfig.TimeKey = "timestamp"
cfg.OutputPaths = append(cfg.OutputPaths, "/var/log/app.log")

logger, err := cfg.Build()
```

---

## Structured Fields

The `zap.Field` type represents a key-value pair for structured logging. Use the typed constructors — they avoid reflection and allocations.

### Primitive Fields

```go
zap.String("name", "Alice")
zap.Int("count", 42)
zap.Int64("id", 1234567890)
zap.Float64("ratio", 0.95)
zap.Bool("enabled", true)
zap.Duration("elapsed", 150*time.Millisecond)
zap.Time("created_at", time.Now())
```

### Pointer Fields (safely handles nil)

```go
name := "Bob"
zap.Stringp("name", &name)   // logs "Bob"
zap.Stringp("name", nil)     // logs null
zap.Intp("count", nil)       // logs null
```

### Slice Fields

```go
zap.Strings("tags", []string{"go", "logging", "zap"})
zap.Ints("ids", []int{1, 2, 3})
zap.Durations("latencies", []time.Duration{1*time.Ms, 2*time.Ms})
```

### Error Fields

```go
err := fmt.Errorf("connection refused")

zap.Error(err)                        // key is always "error"
zap.NamedError("db_error", err)       // custom key
zap.Errors("validation_errors", errs) // slice of errors
```

### Binary and Byte Fields

```go
zap.Binary("payload", rawBytes)       // base64-encoded in JSON
zap.ByteString("utf8_data", textBytes) // treated as a string
```

### Catch-All

```go
// Any picks the best representation automatically (uses reflection as last resort)
zap.Any("value", someUnknownType)
```

### Nested Objects with Dict

```go
logger.Info("request processed",
    zap.Dict("http",
        zap.String("method", "GET"),
        zap.String("path", "/api/users"),
        zap.Int("status", 200),
        zap.Duration("latency", 42*time.Millisecond),
    ),
)
// Output: {"level":"info","msg":"request processed","http":{"method":"GET","path":"/api/users","status":200,"latency":"42ms"}}
```

### Stack Traces

```go
zap.Stack("stacktrace")        // captures current stack from call site
zap.StackSkip("trace", 2)     // skip N frames
```

### Namespaces

```go
logger.With(
    zap.Namespace("database"),
    zap.String("host", "db.example.com"),
    zap.Int("port", 5432),
).Info("connected")
// Output: {..., "database":{"host":"db.example.com","port":5432}}
```

---

## Contextual Logging with `With`

`With` creates a **child logger** that permanently carries the specified fields. The parent is unchanged.

```go
// Create a request-scoped logger
func handleRequest(logger *zap.Logger, r *http.Request) {
    reqLogger := logger.With(
        zap.String("request_id", r.Header.Get("X-Request-ID")),
        zap.String("method", r.Method),
        zap.String("path", r.URL.Path),
        zap.String("remote_addr", r.RemoteAddr),
    )

    reqLogger.Info("request received")

    result, err := processRequest(r)
    if err != nil {
        reqLogger.Error("request failed", zap.Error(err))
        return
    }

    reqLogger.Info("request completed",
        zap.Int("status", 200),
        zap.Duration("duration", time.Since(start)),
    )
}
```

### WithLazy — Deferred Field Evaluation

`WithLazy` is like `With` but serializes the fields **lazily** — only when a log entry is actually written (i.e., passes the level check). Useful for expensive fields.

```go
logger = logger.WithLazy(
    zap.Object("user", expensiveUserObject), // not serialized unless log level passes
)
```

---

## Named Loggers

`Named` appends a name to the logger for easy filtering in log aggregation systems:

```go
baseLogger, _ := zap.NewProduction()

// Create named sub-loggers for each component
httpLogger    := baseLogger.Named("http")
dbLogger      := baseLogger.Named("db")
workerLogger  := baseLogger.Named("worker")

httpLogger.Info("listening")   // {"logger":"http", "msg":"listening"}
dbLogger.Info("connected")    // {"logger":"db",   "msg":"connected"}

// Names are dot-separated
apiLogger := httpLogger.Named("api")
apiLogger.Info("route registered") // {"logger":"http.api", "msg":"route registered"}
```

---

## AtomicLevel — Runtime Level Changes

`AtomicLevel` lets you change the log level **at runtime without restarting** or acquiring locks.

```go
package main

import (
    "net/http"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "os"
)

func main() {
    atom := zap.NewAtomicLevelAt(zap.InfoLevel)

    logger := zap.New(zapcore.NewCore(
        zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
        zapcore.Lock(os.Stdout),
        atom,
    ))
    defer logger.Sync()

    logger.Info("running at info level")
    logger.Debug("this won't print")

    // Change level at runtime
    atom.SetLevel(zap.DebugLevel)
    logger.Debug("now this prints!")

    // AtomicLevel also exposes an HTTP handler for remote control:
    // GET  /log/level          → returns {"level":"debug"}
    // PUT  /log/level?level=info  → changes level to info
    http.Handle("/log/level", atom)
    http.ListenAndServe(":8080", nil)
}
```

Parse a level from a string:

```go
atom, err := zap.ParseAtomicLevel("warn")
// or
atom := zap.NewAtomicLevelAt(zap.WarnLevel)
```

---

## Options

Options are passed to `New`, `NewProduction`, `NewDevelopment`, or `Config.Build`:

```go
logger, _ := zap.NewProduction(
    zap.AddCaller(),                   // include file:line in every entry
    zap.AddCallerSkip(1),             // skip 1 frame (useful in wrapper functions)
    zap.AddStacktrace(zap.WarnLevel), // capture stacktraces at Warn+
    zap.Fields(                        // add fields to every entry
        zap.String("service", "auth"),
        zap.String("env", "production"),
    ),
    zap.WithCaller(true),              // explicitly enable caller info
    zap.Development(),                 // enable development-specific behaviors
)
```

### Hooks

Execute a function after every log entry (useful for metrics, alerting):

```go
logger, _ := zap.NewProduction(
    zap.Hooks(func(entry zapcore.Entry) error {
        if entry.Level >= zap.ErrorLevel {
            alerting.Notify(entry.Message)
        }
        return nil
    }),
)
```

### Fatal Behavior

```go
// Change what happens on Fatal (useful for testing)
logger, _ := zap.NewProduction(
    zap.WithFatalHook(zapcore.WriteThenNoop), // log but don't exit (for tests)
    zap.WithFatalHook(zapcore.WriteThenGoexit), // call runtime.Goexit()
    zap.WithFatalHook(zapcore.WriteThenPanic),  // panic instead of os.Exit
)
```

---

## Advanced Configuration (zapcore)

For complex setups (multi-file output, message queues, different formats per level), use `zapcore` directly:

```go
package main

import (
    "io"
    "os"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func main() {
    // Define level filters
    highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
        return lvl >= zapcore.ErrorLevel
    })
    lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
        return lvl < zapcore.ErrorLevel
    })

    // Define outputs
    consoleOut := zapcore.Lock(os.Stdout)
    consoleErr := zapcore.Lock(os.Stderr)
    fileOut, _, _ := zap.Open("/var/log/app.log")

    // Define encoders
    jsonEncoder    := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
    consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

    // Tee multiple cores together
    core := zapcore.NewTee(
        // Errors → stderr + log file (JSON)
        zapcore.NewCore(jsonEncoder, consoleErr, highPriority),
        zapcore.NewCore(jsonEncoder, fileOut, highPriority),

        // Info/Debug → stdout (human-readable console)
        zapcore.NewCore(consoleEncoder, consoleOut, lowPriority),
    )

    logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
    defer logger.Sync()

    logger.Info("setup complete")    // → stdout (console)
    logger.Error("something broke") // → stderr + file (JSON)
}
```

### Custom Encoder Config

```go
encoderCfg := zapcore.EncoderConfig{
    MessageKey:     "msg",
    LevelKey:       "level",
    TimeKey:        "time",
    NameKey:        "logger",
    CallerKey:      "caller",
    StacktraceKey:  "stacktrace",
    LineEnding:     zapcore.DefaultLineEnding,
    EncodeLevel:    zapcore.LowercaseLevelEncoder,    // "info"
    // EncodeLevel: zapcore.CapitalLevelEncoder,      // "INFO"
    // EncodeLevel: zapcore.CapitalColorLevelEncoder, // colored "INFO"
    EncodeTime:     zapcore.ISO8601TimeEncoder,       // "2006-01-02T15:04:05.000Z0700"
    // EncodeTime:  zapcore.EpochTimeEncoder,         // Unix timestamp float
    // EncodeTime:  zapcore.RFC3339TimeEncoder,       // RFC3339
    EncodeDuration: zapcore.StringDurationEncoder,   // "1.234s"
    // EncodeDuration: zapcore.NanosDurationEncoder, // nanoseconds integer
    EncodeCaller:   zapcore.ShortCallerEncoder,      // "pkg/file.go:42"
    // EncodeCaller: zapcore.FullCallerEncoder,       // full path
}
```

---

## Global Logger

Zap provides global `L()` and `S()` functions as a convenience for codebases that prefer a package-level logger. Replace the globals during initialization:

```go
package main

import "go.uber.org/zap"

func main() {
    logger, _ := zap.NewProduction(
        zap.Fields(zap.String("service", "my-api")),
    )

    // Replace globals — returns an undo function
    undo := zap.ReplaceGlobals(logger)
    defer undo() // restores original no-op loggers

    // Use anywhere in the codebase via package functions:
    zap.L().Info("global logger is set")          // *Logger
    zap.S().Infow("via sugared global", "k", "v") // *SugaredLogger
}
```

> **Tip:** Prefer passing loggers explicitly via dependency injection in larger applications. Global loggers make testing harder.

---

## Standard Library Integration

### Create a `*log.Logger` backed by Zap

```go
logger, _ := zap.NewProduction()

// Wrap zap in a standard log.Logger (at InfoLevel)
stdLog := zap.NewStdLog(logger)
stdLog.Print("this goes through zap")

// Or at a specific level
stdLog, err := zap.NewStdLogAt(logger, zap.DebugLevel)
```

### Redirect the Global Standard Logger

```go
logger, _ := zap.NewProduction()

// Redirect log.Print / log.Printf etc. to zap at InfoLevel
undo := zap.RedirectStdLog(logger)
defer undo() // restore original behavior

// Or redirect at a specific level
undo, err := zap.RedirectStdLogAt(logger, zap.WarnLevel)
```

---

## Custom Objects and Arrays

### Implementing `zapcore.ObjectMarshaler`

```go
type User struct {
    ID    int
    Name  string
    Email string
}

// MarshalLogObject implements zapcore.ObjectMarshaler
func (u User) MarshalLogObject(enc zapcore.ObjectEncoder) error {
    enc.AddInt("id", u.ID)
    enc.AddString("name", u.Name)
    enc.AddString("email", u.Email)
    return nil
}

// Usage:
logger.Info("user created",
    zap.Object("user", User{ID: 1, Name: "Alice", Email: "alice@example.com"}),
)
// Output: {..., "user":{"id":1,"name":"Alice","email":"alice@example.com"}}
```

### Logging a Slice of Objects

```go
users := []User{{1, "Alice", "a@example.com"}, {2, "Bob", "b@example.com"}}

logger.Info("users found",
    zap.Objects("users", users), // Users must implement zapcore.ObjectMarshaler
)
```

### Using `Dict` for Ad-Hoc Objects (no interface needed)

```go
// No interface implementation needed — just compose Fields
logger.Info("payment processed",
    zap.Dict("payment",
        zap.String("id", "pay_abc123"),
        zap.Float64("amount", 99.99),
        zap.String("currency", "USD"),
        zap.String("status", "success"),
    ),
)
```

### Implementing `zapcore.ArrayMarshaler`

```go
type Tags []string

func (t Tags) MarshalLogArray(enc zapcore.ArrayEncoder) error {
    for _, tag := range t {
        enc.AppendString(tag)
    }
    return nil
}

logger.Info("post tagged",
    zap.Array("tags", Tags{"go", "logging", "zap"}),
)
```

---

## Writing to Files

```go
// Open returns a WriteSyncer and a cleanup function
ws, cleanup, err := zap.Open(
    "stdout",              // also write to stdout
    "/var/log/app.log",   // and to a file
)
if err != nil {
    panic(err)
}
defer cleanup()

core := zapcore.NewCore(
    zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
    ws,
    zap.InfoLevel,
)
logger := zap.New(core)
defer logger.Sync()
```

### Custom Sink (e.g., send logs to Kafka)

```go
type KafkaSink struct { /* ... */ }

func (k *KafkaSink) Write(p []byte) (int, error) { /* send to Kafka */ }
func (k *KafkaSink) Sync() error                 { return nil }
func (k *KafkaSink) Close() error                { return nil }

// Register the scheme
zap.RegisterSink("kafka", func(u *url.URL) (zap.Sink, error) {
    return &KafkaSink{brokers: u.Host}, nil
})

// Use in config
cfg := zap.NewProductionConfig()
cfg.OutputPaths = []string{"kafka://kafka-broker:9092/logs"}
logger, _ := cfg.Build()
```

---

## Performance Tips

**Use `Logger.Check` to avoid all work for disabled levels:**

```go
// Check returns nil if the level is disabled — skips field construction entirely
if ce := logger.Check(zap.DebugLevel, "computing expensive log"); ce != nil {
    ce.Write(
        zap.String("result", expensiveComputation()),
        zap.Int("iterations", 1_000_000),
    )
}
```

**Prefer `With` over repeated fields:**

```go
// ✅ Good: create a child logger once with shared context
reqLogger := logger.With(
    zap.String("request_id", reqID),
    zap.String("user_id", userID),
)
reqLogger.Info("started")
reqLogger.Info("finished")

// ❌ Avoid: repeating the same fields on every call
logger.Info("started",  zap.String("request_id", reqID), zap.String("user_id", userID))
logger.Info("finished", zap.String("request_id", reqID), zap.String("user_id", userID))
```

**Always call `Sync` before exit:**

```go
logger, _ := zap.NewProduction()
defer logger.Sync() // flushes any buffered writes — don't skip this!
```

**Use `WithLazy` for expensive fields in hot paths:**

```go
// Fields are only serialized if the log entry actually gets written
logger = logger.WithLazy(
    zap.Object("expensive", computeExpensiveObject()),
)
```

**Prefer typed field constructors over `Any`:**

```go
zap.String("key", val)  // ✅ zero allocation
zap.Any("key", val)     // ❌ may use reflection
```

---

## Performance Benchmarks

From zap's own benchmark suite (log a message with 10 fields):

| Package | Time | Allocations |
|---|---|---|
| ⚡ **zap** | 656 ns/op | 5 allocs/op |
| ⚡ zap (sugared) | 935 ns/op | 10 allocs/op |
| standard library | 124 ns/op\* | 1 allocs/op |
| go-kit | 2,249 ns/op | 57 allocs/op |
| slog | 2,481 ns/op | 42 allocs/op |
| logrus | 11,654 ns/op | 79 allocs/op |

With **pre-existing context** (logger already has 10 fields via `With`):

| Package | Time | Allocations |
|---|---|---|
| ⚡ **zap** | 67 ns/op | 0 allocs/op |
| ⚡ zap (sugared) | 84 ns/op | 1 allocs/op |
| logrus | 10,521 ns/op | 68 allocs/op |

\* *Standard library benchmark is for a static string with no fields.*

---

## Complete Example — Production HTTP Server Logger

```go
package main

import (
    "net/http"
    "time"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func newLogger() (*zap.Logger, error) {
    cfg := zap.NewProductionConfig()

    // Use ISO8601 timestamps instead of Unix epoch
    cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    // Write to both stdout and a log file
    cfg.OutputPaths = []string{"stdout", "/var/log/myapp.log"}

    // Add service-level fields to every entry
    cfg.InitialFields = map[string]any{
        "service": "my-api",
        "version": "2.1.0",
    }

    return cfg.Build(
        zap.AddCaller(),
        zap.AddStacktrace(zap.ErrorLevel),
    )
}

func loggingMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Create a request-scoped logger
        reqLog := logger.With(
            zap.String("method", r.Method),
            zap.String("path", r.URL.Path),
            zap.String("remote_addr", r.RemoteAddr),
            zap.String("request_id", r.Header.Get("X-Request-ID")),
        )

        reqLog.Info("request started")
        next.ServeHTTP(w, r)
        reqLog.Info("request completed", zap.Duration("duration", time.Since(start)))
    })
}

func main() {
    logger, err := newLogger()
    if err != nil {
        panic(err)
    }
    defer logger.Sync()

    // Set as global for convenience
    zap.ReplaceGlobals(logger)

    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    logger.Info("server starting", zap.Int("port", 8080))
    if err := http.ListenAndServe(":8080", loggingMiddleware(logger, mux)); err != nil {
        logger.Fatal("server failed", zap.Error(err))
    }
}
```

---

## Resources

- [Official Docs](https://pkg.go.dev/go.uber.org/zap) — full API reference
- [GitHub Repository](https://github.com/uber-go/zap)
- [FAQ](https://github.com/uber-go/zap/blob/master/FAQ.md) — common questions including design decisions
- [zapcore package](https://pkg.go.dev/go.uber.org/zap/zapcore) — low-level interfaces for extension
