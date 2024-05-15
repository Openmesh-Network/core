package resourcepool

import (
	"context"
	"testing"

	bsclient "github.com/ipfs/boxo/bitswap/client"
	bsnet "github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/openmesh-network/core/updater"
	"github.com/stretchr/testify/assert"
)

func TestAddBlocks(t *testing.T) {
	assert := assert.New(t)
	blockCount := 64

	bsnetwork := bsnet.NewFromIpfsHost(updater.NewHost(), routinghelpers.Null{})
	bstore := blockstore.NewBlockstore(dsync.MutexWrap(datastore.NewMapDatastore()))
	bstore = blockstore.NewIdStore(bstore)

	ctx := context.Background()

	bclient := bsclient.New(ctx, bsnetwork, bstore)

	bservice := blockservice.New(bstore, bclient)

	bmanager := &BlockManager{
		blocksUsed:       0,
		blocksTotal:      blockCount,
		buckets:          make([]cid.Cid, blockCount),
		cidToBucketIndex: make(map[cid.Cid]int),
		bService:         bservice,
		chunkSize:        1,
	}

	{
		b := blocks.NewBlock([]byte{0xff})
		bmanager.AddBlock(ctx, b)
		assert.True(bmanager.blocksUsed == 1)
	}

	{
		bs := make([]blocks.Block, 32)
		for i := range bs {
			bs[i] = blocks.NewBlock([]byte{byte(i)})
		}

		bmanager.AddBlocks(ctx, bs)
		assert.True(bmanager.blocksUsed == 33)
	}

	{ // Don't add identical
		bs := make([]blocks.Block, 32)
		for i := range bs {
			bs[i] = blocks.NewBlock([]byte{byte(i)})
		}

		bmanager.AddBlocks(ctx, bs)
		assert.True(bmanager.blocksUsed == 33)
	}

	{ // Don't go over
		bs := make([]blocks.Block, 32)
		for i := range bs {
			bs[i] = blocks.NewBlock([]byte{byte(i + 33)})
		}

		bmanager.AddBlocks(ctx, bs)
		assert.True(bmanager.blocksUsed == 64)
	}

	{ // Don't go over
		bs := make([]blocks.Block, 64)
		for i := range bs {
			bs[i] = blocks.NewBlock([]byte{byte(i + 65)})
		}

		bmanager.AddBlocks(ctx, bs)
		assert.True(bmanager.blocksUsed == 64)
	}

	{ // Don't go over
		bs := make([]blocks.Block, 64)
		for i := range bs {
			bs[i] = blocks.NewBlock([]byte{byte(i + 97)})
		}

		bmanager.AddBlocks(ctx, bs)
		assert.True(bmanager.blocksUsed == 64)
	}
}
