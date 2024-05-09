package resourcepool

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/openmesh-network/core/networking/p2p"

	"github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"

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

	Bmanager *BlockManager
}

type BlockManager struct {
	// Needs to store the blocks we want and the ones we have.
	// Need a way to capture the total storage available to us.
	// This dude should wrap the blockservice probably.

	blocksUsed       int
	blocksTotal      int
	buckets          []cid.Cid
	cidToBucketIndex map[cid.Cid]int // 0 index is invalid.
	bService         blockservice.BlockService
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

	blockCount := 64
	inst.Bmanager = &BlockManager{
		blocksUsed:       1,
		blocksTotal:      blockCount,
		buckets:          make([]cid.Cid, blockCount),
		cidToBucketIndex: make(map[cid.Cid]int),
		bService:         inst.Bservice,
	}
}

func (inst *Instance) Stop() {
	inst.Bsnetwork.Stop()
}

// TODO:
// - Add mutex to avoid race condition.
// - Make this work with many blocks (Actual common case).
func (bm *BlockManager) AddBlockWithFixedChunk(ctx context.Context, b blocks.Block) {
	// Basic sanity check
	if len(b.RawData()) != DEFAULT_CHUNK_SIZE {
		panic("Block doesn't match raw data size. ERROR.")
	}

	// Check if it's already been added.
	if bm.cidToBucketIndex[b.Cid()] != 0 {
		return
	}

	// Check the allocated space can fit this new block.
	if bm.blocksTotal-bm.blocksTotal > 0 {
		found := false
		for i := range bm.buckets {
			if bm.buckets[i].Bytes() == nil {
				bm.buckets[i] = b.Cid()
				bm.cidToBucketIndex[b.Cid()] = i + 1
				bm.bService.AddBlock(ctx, b)

				found = true
			}
		}

		if !found {
			panic("This should never happen. There's blockTotal - blockUsed.")
		}
	} else {
		// If not, work out what to delete or if to delete anything.
		// Delete older stuff and keep newer stuff.
		// For now assume all blocks are equal.

		{ // Should consider deleting any blocks which have been safely upladed maybe?
		}

		{ // Maybe delete blocks that are
		}

		{ // First approach, just delete oldest blocks. Simple treadmill.
			// Take out first in array since it is oldest and shuffle everything back.
			// Flaws:
			//	- Once we reach the maximum number of nodes we have to do this once for every single new addition.
			//	  Ideally we'd want to avoid doing this as much as is possible.
			//	- Doesn't take into account the kind of block we're storing.

			// XXX: Might have to replace with linked list, since we coudl be dealing with thousands of items here.
			// Possible optimization.
			oldCid := bm.buckets[0]
			bm.bService.DeleteBlock(ctx, oldCid)

			copy(bm.buckets[:], bm.buckets[1:])
			bm.buckets[len(bm.buckets)-1] = b.Cid()

			// Have to do this since all indeces are changed now.
			// XXX: Might have to replace this for performance.
			for i, c := range bm.buckets {
				bm.cidToBucketIndex[c] = i + 1
			}

			bm.blocksTotal -= 1
		}
	}

	// TODO: Ignore blocks with the CID that's all 0.
}
