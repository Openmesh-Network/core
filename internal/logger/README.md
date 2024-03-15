# Package Logger

This is the package for logging (based on `zap`).

## Usage

```go
// Initialise a logger first
logger.InitLogger()
// Use sugared logger
logger.Infof("Message: %s", msg)
// Don't forget to sync your logs when you log to file
defer logger.SyncAll()
```

## Configuration

```yaml
log:
  # development mode is more human-friendly: console encoding and not log to file
  development: true
  # THE FOLLOWING CONFIGS ONLY WORKS IN THE PRODUCTION MODE (development: false)
  # console or json
  encoding: console
  info:
    # whether log to stdout or not
    toStdout: true
    # whether log to file or not
    toFile: true
    # path and name of the INFO-level log file
    fileName: logs/core-info.log
    # maximum size of a single INFO-level log file (megabytes)
    maxSize: 10
    # how many days old INFO-level log files to retain
    maxAge: 7
    # how much old INFO-level log files to retain
    maxBackups: 10
  error:
    # whether log to stderr or not
    toStderr: true
    # whether log to file or not
    toFile: true
    # path and name of the ERROR-level log file
    fileName: logs/core-error.log
    # maximum size of a single ERROR-level log file (megabytes)
    maxSize: 10
    # how many days old ERROR-level log files to retain
    maxAge: 7
    # how much old ERROR-log files to retain
    maxBackups: 10

```

## Supported log levels

From [Log levels in Zap](https://betterstack.com/community/guides/logging/go/zap/#log-levels-in-zap):

- `DEBUG (-1)`:
  - Usage: `logger.Debug()` or `zap.L().Debug()`.
  - For recording messages useful for debugging.
- `INFO (0)`:
  - Usage: `logger.Info()` or `zap.L().Info()`.
  - For messages describing normal application operations.
- `WARN (1)`: 
  - Usage: `logger.Warn()` or `zap.L().Warn()`.
  - For recording messages indicating something unusual happened that may need attention before it escalates to a more severe issue.
- `ERROR (2)`: 
  - Usage: `logger.Error()` or `zap.L().Error()`
  - For recording unexpected error conditions in the program.
- `DPANIC (3)`:
  - Usage: `logger.DPanic()` or `zap.L().DPanic()`.
  - For recording severe error conditions in development. It behaves like PANIC in development and ERROR in production.
- `PANIC (4)`:
  - Usage: `logger.Panic()` or `zap.L().Panic()`.
  - Calls panic() after logging an error condition.
- `FATAL (5)`:
  - Usage: `logger.Fatal()` or `zap.L().Fatal()`.
  - Calls os.Exit(1) after logging an error condition.

## Sugared logger

Sugared logger is more human-friendly and have a syntax similar to `fmt.Print()` and `fmt.Printf()`.

```go
// Use sugared logger
logger.Info("This is sugared logger!!!")
logger.Infof("Message: %s", msg)
logger.Errorf("Failed to do bar: %s", err.Error())
```

Supported formats:

- `Print()`-like (e.g., `Info()`)
- `Printf()`-like (e.g., `Infof()`)
- `Println()`-like (e.g., `Infoln()`)
- Non-sugared logger style (use key-value pairs, e.g., `Infow()`)

## Non-sugared logger

Non-sugared logger has a better performance and is more suitable for performance sensitive scenario.

```go
// Use non-sugared logger
zap.L().Info("This is message", zap.String("key", "value"))
zap.L().Error("Failed to do foo", zap.String("error", err.Error()))
```

## Log format

The following two formats are supported:

- `json` (default): Default format that is better for parsing and processing. Example: `{"level":"info","timestamp":"2024-03-15T12:29:34.657+1100","msg":"This is test logger: INFO level"}`
- `console`: A more human-friendly format. Example: `2024-03-15T12:30:13.390+1100	INFO	This is test logger: INFO level`
