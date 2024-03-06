package main

import (
    "context"
    "log"
    "openmesh.network/openmesh-core/internal/config"
    "openmesh.network/openmesh-core/internal/core"
    "openmesh.network/openmesh-core/internal/networking/p2p"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    config.ParseFlags()
    config.ParseConfig()

    // Initialise graceful shutdown
    cancelCtx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Initialise p2p instance
    p2pInstance, err := p2p.NewInstance(cancelCtx).Build()
    if err != nil {
        log.Fatalf("Failed to initialise p2p instance: %s", err.Error())
    }

    // Build and start top-level instance
    core.NewInstance().SetP2pInstance(p2pInstance).Start()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

    // Stop here!
    sig := <-sigChan
    log.Printf("Termination signal received: %v", sig)
}
