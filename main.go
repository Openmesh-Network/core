package main

import (
	"context"
	_ "embed"
	"os"
	"os/signal"
	"syscall"

	"net/http"
	_ "net/http/pprof"
	"runtime/pprof"

	"github.com/openmesh-network/core/bft"
	"github.com/openmesh-network/core/collector"
	"github.com/openmesh-network/core/config"
	"github.com/openmesh-network/core/internal/core"
	"github.com/openmesh-network/core/internal/logger"
	"github.com/openmesh-network/core/networking/p2p"
	rp "github.com/openmesh-network/core/resourcepool"
	"github.com/openmesh-network/core/updater"
)

const (
	useRuntimeConfigFile = true
	debugMinimalBuild    = false
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
	if useRuntimeConfigFile {
		config.ParseFlags()
	}
	config.ParseConfig(configCompileValue, useRuntimeConfigFile)

	if config.Config.Prof.Enable {

		if config.Config.Prof.EnableHttp {
			go http.ListenAndServe("localhost:8080", nil)
		}

		f, err := os.Create(config.Config.Prof.FileName)
		if err != nil {
			panic(err)
		}

		pprof.StartCPUProfile(f)
	}
	defer pprof.StopCPUProfile()

	// Initialise logger after parsing configuration
	logger.InitLogger()
	defer logger.SyncAll()

	// Initialise graceful shutdown.
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialise p2p instance.
	var p2pInstance *p2p.Instance
	var err error

	p2pInstance, err = p2p.NewInstance(cancelCtx, config.Config.P2P).Build()
	if err != nil {
		logger.Fatalf("Failed to initialise p2p instance: %s", err.Error())
	}

	rpInstance := rp.NewInstance(p2pInstance)
	rpInstance.Start(cancelCtx)
	defer rpInstance.Stop()

	// Need collector before bft.
	var collectorInstance *collector.CollectorInstance
	if debugMinimalBuild {
		collectorInstance = nil
	} else {
		collectorInstance = collector.NewInstance(rpInstance)
		collectorInstance.Start(cancelCtx)
	}

	// Initialise CometBFT instance
	bftInstance, err := bft.NewInstance(collectorInstance)
	if err != nil {
		logger.Fatalf("Failed to initialise CometBFT instance: %s", err.Error())
	}

	// Run the updater.
	// TODO: Maybe pass past CID versions to avoid redownloading old updates.
	if debugMinimalBuild {
	} else {
		updater.NewInstance(TrustedKeys, p2pInstance).Start(cancelCtx)
	}

	// Build and start top-level instance.
	ins := core.NewInstance().
		SetP2pInstance(p2pInstance).
		SetBFTInstance(bftInstance)
	ins.Start(cancelCtx)
	logger.Infof("Openmesh Core started successfully.")
	defer ins.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// Stop here!
	sig := <-sigChan
	logger.Infof("Termination signal received: %v", sig)
}
