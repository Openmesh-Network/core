package main

import (
	"context"
	"github.com/openmesh-network/core/internal/config"
	"github.com/openmesh-network/core/internal/core"
	"github.com/openmesh-network/core/internal/logger"
	"github.com/openmesh-network/core/internal/networking/p2p"
	"github.com/openmesh-network/core/updater"
	"os"
	"os/signal"
	"syscall"
)

// These are the public keys trusted to sign new updates.
var TrustedKeys = []updater.PublicKey{
	// XXX: THESE ARE NOT THE FINAL KEYS, CHANGE BEFORE DEPLOYING TO PRODUCTION!!!
	updater.PublicKeyFromBase64("HJOvRAmk3tYFvs2uFm+06T6kU9MC2oT+8s1Scwqf224"),
	// updater.PublicKeyFromBase64("jt1/Mb2xWnd7z6pn21iTb9EU4wycdZhT6Zgb3xf+h6k"),
	// updater.PublicKeyFromBase64("+8rZEcO928jPGlkn0CZKbXxi11twmZbj9KxxBvTa15Q"),
	// Fake key
	// updater.PublicKeyFromBase64("JZlpAGC7aYXIupMUQN48daT/tYRulWiOC0sXFNEXFNE"),
}

func main() {
	config.ParseFlags()
	config.ParseConfig()

	// Initialise logger after parsing configuration
	logger.InitLogger()
	defer logger.SyncAll()

	// Initialise graceful shutdown.
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialise p2p instance.
	p2pInstance, err := p2p.NewInstance(cancelCtx).Build()
	if err != nil {
		logger.Fatalf("Failed to initialise p2p instance: %s", err.Error())
	}

	// Run the updater.
	// TODO: Maybe pass past CID versions to avoid redownloading old updates.
	updater.NewInstance(TrustedKeys, p2pInstance).Start(cancelCtx)

	// Build and start top-level instance.
	ins := core.NewInstance().SetP2pInstance(p2pInstance)
	ins.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// Stop here!
	sig := <-sigChan
	logger.Infof("Termination signal received: %v", sig)
	ins.Stop()
}
