package collector

import (
	"context"
	"testing"

	"github.com/openmesh-network/core/internal/config"
	"github.com/openmesh-network/core/networking/p2p"
	"github.com/openmesh-network/core/resourcepool"
)

var dummysource = Source{
	"dummysource",
	dummyjoin,
	"", []string{""},
	"",
}

var dummyrequest = Request{
	dummysource,
	0,
}


func dummyjoin(ctx context.Context, source Source, topic string) (chan []byte, <-chan error, error) {
	return nil, nil, nil
}

func TestBasic(t *testing.T) {
	ctx := context.Background()

	p2pConfig := config.P2pConfig{}

	ci := NewInstance(resourcepool.NewInstance(p2p.NewInstance(ctx, p2pConfig)))

	ci.Start(ctx)

	ci.SubmitRequests()

}
