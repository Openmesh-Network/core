# Package Logger

This is the package for logging (based on `zap`).

## Usage

```go
// Initialise a logger first
logger.InitLogger()
// Use sugared logger
logger.Infof("Message: %s", msg)
```

## Sugared Logger

Sugared logger is more human-friendly and have a syntax similar to `fmt.Print()` and `fmt.Printf()`.

```go
// Use sugared logger
logger.Info("This is sugared logger!!!")
logger.Infof("Message: %s", msg)
logger.Errorf("Failed to do bar: %s", err.Error())
```

## Non-sugared Logger

Non-sugared logger has a better performance and is more suitable for performance sensitive scenario.

```go
// Use non-sugared logger
zap.L().Info("This is message", zap.String("key", "value"))
zap.L().Error("Failed to do foo", zap.String("error", err.Error()))
```
