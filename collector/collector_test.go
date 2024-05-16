package collector

import (
	"context"
	"encoding/binary"
	"math/rand"
	"testing"
	"time"

	"github.com/minio/sha256-simd"
	"github.com/openmesh-network/core/config"
	log "github.com/openmesh-network/core/logger"
	"github.com/openmesh-network/core/networking/p2p"
	"github.com/openmesh-network/core/resourcepool"
)

// var dummysource = Source{
// 	"dummysource",
// 	dummyjoin,
// 	"", []string{""},
// 	"",
// }

// var dummyrequest = Request{
// 	dummysource,
// 	0,
// }

func TestBasic(t *testing.T) {
	ctx := context.Background()

	p2pConfig := config.P2pConfig{}

	ci := NewInstance(resourcepool.NewInstance(p2p.NewInstance(ctx, p2pConfig)))

	ci.Start(ctx)

	// ci.SubmitRequests()

}

func TestAnchoring(t *testing.T) {
	// Run a few nodes and see if the CIDs match.

	ctx := context.Background()

	p2pConfig := config.P2pConfig{}
	p2pConfig.Addr = "0.0.0.0"
	p2pConfig.Port = 0
	p2pConfig.GroupName = "xnode"
	p2pConfig.PeerLimit = 50

	ncollectors := 20
	cis := make([]*CollectorInstance, ncollectors)
	log.InitLogger()

	p, err := p2p.NewInstance(ctx, p2pConfig).Build()
	if err != nil {
		panic(err)
	}

	p.Start()
	time.Sleep(time.Millisecond * 200)

	rp := resourcepool.NewInstance(p)
	rp.Start(ctx)

	// Make more than one, and change the timeout to simulate the
	for i := 0; i < ncollectors; i++ {
		// Everyone gets the SAME resource pool
		cis[i] = NewInstance(rp)
		cis[i].Start(ctx)
	}

	requestCurrent := [1]Request{}
	requestNext := [1]Request{}

	requestCurrent[0].Source = Sources[3]
	requestCurrent[0].Topic = 0
	requestNext[0].Source = Sources[3]
	requestNext[0].Topic = 1

	ciSummaries := make([]Summary, ncollectors)

	for round := 0; round < 5; round++ {
		t.Log("Starting round: ", round)

		targetTime := time.Now().Add(-time.Second)
		hash := sha256.Sum256([]byte{byte(round), byte(targetTime.Unix())})
		hashInt := binary.LittleEndian.Uint64(hash[:8])
		for i := 0; i < ncollectors; i++ {
			index := i

			go func() {
				summaries := cis[index].SubmitRequests(requestCurrent[:], requestNext[:], targetTime.Add(time.Millisecond*time.Duration(400.0*(rand.Float32()*2-1))), hashInt)
				if len(summaries) > 0 {
					ciSummaries[index] = summaries[0]
				}
			}()
		}

		t.Log("Sleeping...")
		time.Sleep(time.Second * 8)

		t.Log("Summaries: ")

		counts := make(map[string]int)
		tally := 0
		for i := 0; i < ncollectors; i++ {
			if len(ciSummaries[i].DataHashes) > 0 {
				key := ciSummaries[i].DataHashes[0].String()
				_, ok := counts[key]

				if ok {
					counts[key] += 1
				} else {
					counts[key] = 1
				}
				tally++
			}
		}

		for k, v := range counts {
			t.Log("\t", k, ":", v)
		}
		t.Log("Tally: ", tally)

		t.Log("Next round...")
		temp := requestCurrent
		requestCurrent = requestNext
		requestNext = temp
	}

}
