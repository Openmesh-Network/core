package logger

import (
    "go.uber.org/zap"
    "openmesh.network/openmesh-core/internal/config"
)

var (
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
    logger := zap.Must(zap.NewProduction())
    if config.Config.Log.Development {
        logger = zap.Must(zap.NewDevelopment())
    }
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
