package main

import (
	"context"
	_ "embed"
	"os"
	"os/signal"
	"syscall"

	"github.com/openmesh-network/core/internal/bft"
	"github.com/openmesh-network/core/internal/config"
	"github.com/openmesh-network/core/internal/core"
	"github.com/openmesh-network/core/internal/database"
	"github.com/openmesh-network/core/internal/logger"
	"github.com/openmesh-network/core/networking/p2p"
	"github.com/openmesh-network/core/updater"
)

const (
	allowLoadConfigAtRuntime = true
)

var (
	// These are the public keys trusted to sign new updates.
	TrustedKeys = []updater.PublicKey{
		// XXX: THESE ARE NOT THE FINAL KEYS, CHANGE BEFORE DEPLOYING TO PRODUCTION!!!
		updater.PublicKeyFromBase64("HJOvRAmk3tYFvs2uFm+06T6kU9MC2oT+8s1Scwqf224"),
		// updater.PublicKeyFromBase64("jt1/Mb2xWnd7z6pn21iTb9EU4wycdZhT6Zgb3xf+h6k"),
		// updater.PublicKeyFromBase64("+8rZEcO928jPGlkn0CZKbXxi11twmZbj9KxxBvTa15Q"),
		// Fake key
		// updater.PublicKeyFromBase64("JZlpAGC7aYXIupMUQN48daT/tYRulWiOC0sXFNEXFNE"),
	}
	//go:embed config.yml
	configCompileValue string
)

func main() {
	if allowLoadConfigAtRuntime {
		config.ParseFlags()
	}
	config.ParseConfig(configCompileValue, allowLoadConfigAtRuntime)

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

	// Initialise BadgerDB connection
	dbInstance, err := database.NewInstance()
	if err != nil {
		logger.Fatalf("Failed to establish PostgreSQL connection: %s", err.Error())
	}

	// Initialise CometBFT instance

	bftInstance, err := bft.NewInstance(dbInstance.Conn)
	if err != nil {
		logger.Fatalf("Failed to initialise CometBFT instance: %s", err.Error())
	}

	// Run the updater.
	// TODO: Maybe pass past CID versions to avoid redownloading old updates.
	updater.NewInstance(TrustedKeys, p2pInstance).Start(cancelCtx)

	// Build and start top-level instance.
	ins := core.NewInstance().
		SetP2pInstance(p2pInstance).
		SetDBInstance(dbInstance).
		SetBFTInstance(bftInstance)
	ins.Start()
	logger.Infof("Openmesh Core started successfully.")
	defer ins.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// Stop here!
	sig := <-sigChan
	logger.Infof("Termination signal received: %v", sig)
}
