package resourcepool

import (
	"context"
	"testing"
	"time"

	bsclient "github.com/ipfs/boxo/bitswap/client"
	bsnet "github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
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

func TestQueryIPFSBlocks(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	h := updater.NewHost()
	// Use bootstrapping to find node or use a reliable public node
	ipfsNode := "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWSN7WdZFAPk3313uxTYapxk4Lhc974CkNDu9Ke2dRiNrA"
	addr, err := peer.AddrInfoFromP2pAddr(multiaddr.StringCast(ipfsNode))
	assert.Nil(err)

	bsnetwork := bsnet.NewFromIpfsHost(h, routinghelpers.Null{})
	bstore := blockstore.NewBlockstore(dsync.MutexWrap(datastore.NewMapDatastore()))
	bstore = blockstore.NewIdStore(bstore)
	bclient := bsclient.New(ctx, bsnetwork, bstore)
	bsnetwork.Start(bclient)

	err = h.Connect(ctx, *addr)
	assert.Nil(err)

	{
		queryCid := cid.MustParse("bafkreihunxxowxpx2zzmswyiddnswyqgxooij3eg7o5j4g2mbnv7vwk4ya")
		block, err := bclient.GetBlock(ctx, queryCid)
		assert.Nil(err)
		assert.Equal(`{"title":"Test","tags":[{"tag":"Test"}],"projectSize":0,"teamSize":0,"description":"<p>Test</p>","resources":"","links":[]}`, string(block.RawData()[:]))
	}
}
