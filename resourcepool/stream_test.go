package resourcepool

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/openmesh-network/core/config"
	log "github.com/openmesh-network/core/logger"
	"github.com/openmesh-network/core/networking/p2p"
)

func setupTest() *Instance {
	ctx := context.Background()

	p2pConfig := config.P2pConfig{}
	p2pConfig.Addr = "0.0.0.0"
	p2pConfig.Port = 0
	p2pConfig.GroupName = "xnode"
	p2pConfig.PeerLimit = 50

	log.InitLogger()

	p, err := p2p.NewInstance(ctx, p2pConfig).Build()
	if err != nil {
		panic(err)
	}

	p.Start()
	time.Sleep(time.Millisecond * 200)

	rp := NewInstance(p)
	rp.Start(ctx)

	return rp
}

func TestStream(t *testing.T) {
	s := setupTest().NewStream()

	check := []int{32, 64, 128, 1024, 2048, 4096, 4096*2 + 2}
	for _, c := range check {

		buf := make([]byte, c)
		total := time.Millisecond * 0

		for run := 0; run < 10; run++ {
			start := time.Now()

			for j := 0; j < c; j++ {
				buf[j] = byte(rand.Uint64())
			}

			s.Append(buf)

			total += time.Now().Sub(start)
			s.Reset()
		}

		t.Log("Size: ", c, "Average time: ", total.Microseconds()/10, "Total time:", total.Microseconds())
	}
}

func TestLargeMessage(t *testing.T) {
	s := setupTest().NewStream()
	buf := make([]byte, DEFAULT_CHUNK_SIZE*8)

	for i := range buf {
		buf[i] = byte(rand.Uint64())
	}

	s.Append(buf)
}

func TestEncodeDecode(t *testing.T) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		panic(err)
	}

	rp := setupTest()
	s := rp.NewStream()

	original := make([]byte, DEFAULT_CHUNK_SIZE*5)

	r := rand.New(rand.NewSource(1))
	for i := range original {
		original[i] = byte(r.Uint64())
	}
	s.Append(original)

	dst := make([]byte, 0, len(original))

	// Get every CID
	for _, c := range s.cidHashes {
		block, err := rp.Bservice.GetBlock(context.Background(), c)
		if err != nil {
			panic(err)
		}

		buf := make([]byte, 0, len(original))
		buf, err = dec.DecodeAll(block.RawData(), buf)
		t.Log("Passed first!")

		if err != nil {
			// fmt.Printf("%x", buf)
			fmt.Printf("No Magicks :( : %x", block.RawData())
			panic(err)
		} else {
		}

		dst = append(dst, buf...)
	}

	dst, err = dec.DecodeAll(s.compressedBuffer, dst)
	if err != nil {
		panic(err)
	}

	t.Log("Dst:", len(dst))
	t.Log("Compressed:", len(s.compressedBuffer))
	t.Log("Original:", len(original))

	if len(dst) != len(original) {
		panic("Different lengths between original and uncompressed!")
	}

	for i := range dst {
		if dst[i] != original[i] {
			t.Log(original)
			t.Log(dst)
			panic("Error result doesn't match after compression.")
		}
	}
}
