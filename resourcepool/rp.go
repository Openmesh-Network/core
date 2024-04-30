package resourcepool

import (
	"context"

	"github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/openmesh-network/core/networking/p2p"

	"github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"

	// offline "github.com/ipfs/boxo/exchange/offline"

	// blocks "github.com/ipfs/go-block-format"

	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"

	bsclient "github.com/ipfs/boxo/bitswap/client"
	bsnet "github.com/ipfs/boxo/bitswap/network"
	bsserver "github.com/ipfs/boxo/bitswap/server"
)

const DEFAULT_CHUNK_SIZE = 4096

type Instance struct {
	Bservice  blockservice.BlockService
	Bstore    blockstore.Blockstore
	Bsnetwork bsnet.BitSwapNetwork
	Bsserver  *bsserver.Server
	Bsclient  *bsclient.Client
	host      *host.Host
}

func NewInstance(p2pinst *p2p.Instance) *Instance {
	var inst Instance

	inst.host = p2pinst.Host

	inst.Bsnetwork = bsnet.NewFromIpfsHost(*inst.host, routinghelpers.Null{})

	// TODO: Make a custom blockstore that we can fully control.
	inst.Bstore = blockstore.NewBlockstore(dsync.MutexWrap(datastore.NewMapDatastore()))
	inst.Bstore = blockstore.NewIdStore(inst.Bstore)

	return &inst
}

func (inst *Instance) Start(ctx context.Context) {
	inst.Bsclient = bsclient.New(ctx, inst.Bsnetwork, inst.Bstore)
	inst.Bsserver = bsserver.New(ctx, inst.Bsnetwork, inst.Bstore)

	inst.Bservice = blockservice.New(inst.Bstore, inst.Bsclient)
	inst.Bsnetwork.Start(inst.Bsclient, inst.Bsserver)
}

func (inst *Instance) Stop() {
	inst.Bsnetwork.Stop()
}
