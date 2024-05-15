package tracker

import (
	"context"
	"time"

	validatorpass_tracker "github.com/Openmesh-Network/nft-authorise/tracker"
	log "github.com/openmesh-network/core/internal/logger"

	"github.com/openmesh-network/core/config"
)

type Instance struct {
	Tracker *validatorpass_tracker.Tracker
}

func NewInstance() (*Instance, error) {
	i := &Instance{}
	log.Error("the rpc address is", config.Config)
	trackerobj := validatorpass_tracker.NewTracker(config.Config.Nft.RpcAddress, config.Config.Nft.SearchLimit, validatorpass_tracker.NewRedeemEvent("Redeemed(uint256,bytes32)", "0x8D64aB58a17dA7d8788367549c513386f09a0A70", config.Config.Nft.DeployBlock))
	i.Tracker = trackerobj
	return i, nil
}

// Stop the CometBFT node
func (i *Instance) Start(ctx context.Context) {
	go i.Tracker.StartTracking(ctx, time.Second*time.Duration(config.Config.Nft.Timing), config.Config.Nft.Confirmations)
	msg := <-i.Tracker.Startsig
	if msg == "done" {
		abc := i.Tracker.LastTrackerHeight
		log.Error("height is blah", abc)
	}

}
