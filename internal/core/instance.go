package core

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
	"github.com/openmesh-network/core/bft"
	"github.com/openmesh-network/core/config"
	"github.com/openmesh-network/core/database"
	"github.com/openmesh-network/core/logger"
	"github.com/openmesh-network/core/networking/p2p"
	"github.com/openmesh-network/core/tracker"
)

// Instance is the top-level instance
type Instance struct {
	pi      *p2p.Instance
	DB      *database.Instance
	BFT     *bft.Instance
	Tracker *tracker.Instance
}

// NewInstance initialise an empty top-level instance
func NewInstance() *Instance {
	return &Instance{}
}

func (i *Instance) SetP2pInstance(pi *p2p.Instance) *Instance {
	i.pi = pi
	return i
}

func (i *Instance) SetDBInstance(db *database.Instance) *Instance {
	i.DB = db
	return i
}

func (i *Instance) SetBFTInstance(bft *bft.Instance) *Instance {
	i.BFT = bft
	return i
}
func (i *Instance) SetTrackerInstance(tracker *tracker.Instance) *Instance {
	i.Tracker = tracker
	return i
}

// Start the top-level instance as well as all the low-level instances
func (i *Instance) Start(ctx context.Context) {
	err := i.pi.Start()
	if err != nil {
		logger.Fatalf("Failed to start p2p instance: %s", err.Error())
	}

	if config.Config.P2P.DebugAutoconnectMultiaddr != "" {
		logger.Debug("Connecting to libp2p multiaddress in config...")
		i.pi.ConnectFromMultiaddr(ctx, config.Config.P2P.DebugAutoconnectMultiaddr)
	}

	log.Error("Starting BFT")
	i.BFT.Start(ctx)

}

// Stop the top-level instance as well as all the low-level instances
func (i *Instance) Stop() {
	if err := i.pi.Stop(); err != nil {
		logger.Errorf("Failed to stop p2p instance: %s", err.Error())
	}

	if err := i.BFT.Stop(); err != nil {
		logger.Errorf("Failed to stop CometBFT instance: %s", err.Error())
	}
}
