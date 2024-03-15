package logger

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "gopkg.in/natefinch/lumberjack.v2"
    "openmesh.network/openmesh-core/internal/config"
    "os"
)

var (
    syncs    []func() error
    Debug    func(...interface{})
    Debugf   func(string, ...interface{})
    Debugw   func(string, ...interface{})
    Debugln  func(...interface{})
    Info     func(...interface{})
    Infof    func(string, ...interface{})
    Infow    func(string, ...interface{})
    Infoln   func(...interface{})
    Warn     func(...interface{})
    Warnf    func(string, ...interface{})
    Warnw    func(string, ...interface{})
    Warnln   func(...interface{})
    Error    func(...interface{})
    Errorf   func(string, ...interface{})
    Errorw   func(string, ...interface{})
    Errorln  func(...interface{})
    DPanic   func(...interface{})
    DPanicf  func(string, ...interface{})
    DPanicw  func(string, ...interface{})
    DPanicln func(...interface{})
    Panic    func(...interface{})
    Panicf   func(string, ...interface{})
    Panicw   func(string, ...interface{})
    Panicln  func(...interface{})
    Fatal    func(...interface{})
    Fatalf   func(string, ...interface{})
    Fatalw   func(string, ...interface{})
    Fatalln  func(...interface{})
)

// InitLogger initialise a logger and assign it to the global variables
func InitLogger() {
    stdout := zapcore.AddSync(os.Stdout)
    infoLevel := zap.NewAtomicLevelAt(zap.InfoLevel)
    infoConfig := config.Config.Log.InfoConfig
    infoFile := zapcore.AddSync(&lumberjack.Logger{
        Filename:   infoConfig.FileName,
        MaxSize:    infoConfig.MaxSize,
        MaxAge:     infoConfig.MaxAge,
        MaxBackups: infoConfig.MaxBackups,
        LocalTime:  true,
    })

    stderr := zapcore.AddSync(os.Stderr)
    errLevel := zap.NewAtomicLevelAt(zap.ErrorLevel)
    errConfig := config.Config.Log.ErrorConfig
    errFile := zapcore.AddSync(&lumberjack.Logger{
        Filename:   errConfig.FileName,
        MaxSize:    errConfig.MaxSize,
        MaxAge:     errConfig.MaxAge,
        MaxBackups: errConfig.MaxBackups,
        LocalTime:  true,
    })

    // Initialise log config based on development/production config
    cfg := zap.NewProductionEncoderConfig()
    cfg.TimeKey = "timestamp"
    cfg.EncodeTime = zapcore.ISO8601TimeEncoder
    if config.Config.Log.Development {
        cfg = zap.NewDevelopmentEncoderConfig()
        cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
    }
    encoder := getEncoder(config.Config.Log.Encoding, cfg)

    // Add log configs to the core based on the configuration provided
    cores := make([]zapcore.Core, 0)
    if infoConfig.ToFile {
        cores = append(cores, zapcore.NewCore(encoder, infoFile, infoLevel))
        syncs = append(syncs, infoFile.Sync)
    }
    if infoConfig.ToStdout {
        cores = append(cores, zapcore.NewCore(encoder, stdout, infoLevel))
    }
    if errConfig.ToFile {
        cores = append(cores, zapcore.NewCore(encoder, errFile, errLevel))
        syncs = append(syncs, errFile.Sync)
    }
    if errConfig.ToStderr {
        cores = append(cores, zapcore.NewCore(encoder, stderr, errLevel))
    }

    // Initialise Zap logger
    core := zapcore.NewTee(cores...)
    logger := zap.New(core)
    zap.ReplaceGlobals(logger)

    // Convert it to a sugared logger to use something like Infof() and Info()
    sugar := logger.Sugar()

    // Assign global variables
    // DEBUG level (-1)
    Debug = sugar.Debug
    Debugf = sugar.Debugf
    Debugw = sugar.Debugw
    Debugln = sugar.Debugln

    // INFO level (0)
    Info = sugar.Info
    Infof = sugar.Infof
    Infow = sugar.Infow
    Infoln = sugar.Infoln

    // WARN level (1)
    Warn = sugar.Warn
    Warnf = sugar.Warnf
    Warnw = sugar.Warnw
    Warnln = sugar.Warnln

    // ERROR level (2)
    Error = sugar.Error
    Errorf = sugar.Errorf
    Errorw = sugar.Errorw
    Errorln = sugar.Errorln

    // DPANIC level (3)
    DPanic = sugar.DPanic
    DPanicf = sugar.DPanicf
    DPanicw = sugar.DPanicw
    DPanicln = sugar.DPanicln

    // PANIC level (4)
    Panic = sugar.Panic
    Panicf = sugar.Panicf
    Panicw = sugar.Panicw
    Panicln = sugar.Panicln

    // FATAL level (5)
    Fatal = sugar.Fatal
    Fatalf = sugar.Fatalf
    Fatalw = sugar.Fatalw
    Fatalln = sugar.Fatalln
}

// SyncAll sync all the files added to the logger
func SyncAll() error {
    for _, sync := range syncs {
        err := sync()
        if err != nil {
            return err
        }
    }
    return nil
}

// getEncoder create and return a proper encoder based on the format specified
func getEncoder(format string, c zapcore.EncoderConfig) zapcore.Encoder {
    switch format {
    case "json":
        return zapcore.NewJSONEncoder(c)
    case "console":
        return zapcore.NewConsoleEncoder(c)
    default:
        return zapcore.NewJSONEncoder(c)
    }
}
