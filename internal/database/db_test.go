package database

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/openmesh-network/core/internal/config"
)

// Mock data for testing
func runAllTests(t *testing.T) {

}

// Actual unit tests
// Test the storeMessageChunk function.
func TestStoreCID(t *testing.T) {
	config.ParseFlags()
	config.Path = "../../"
	config.ParseConfig(config.Path, true)

	//c, cancel := context.WithCancel(context.Background())
	//defer cancel()

	// Try to reference the CID from bdb, test output against input and readability of the if reconstructed.
}

// Helper Functions
// Used as a test for key-value pairs that should be stored correctly and retrievably in BadgerDB.
type testCid struct {
	cid  cid.Cid
	data []byte
}

func makeCidFromString(input string) testCid {
	cidBuilder := cid.V1Builder{
		Codec:    uint64(multicodec.DagPb),
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: -1,
	}
	c, err := cidBuilder.Sum([]byte(input))
	if err != nil {
		// If this fails to parse a buffer the input is invalid.
		panic(err)
	}
	return testCid{
		cid:  c,
		data: []byte(input),
	}
}

// Used to generate mock key-value pairs within this test.
func generateCID(data []byte) cid.Cid {
	cidBuilder := cid.V1Builder{
		Codec:    uint64(multicodec.DagPb),
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: -1,
	}
	c, err := cidBuilder.Sum(data)
	if err != nil {
		// If this fails to parse a buffer the input is invalid.
		panic(err)
	}
	return c
}

// Will use to validate the CID output of another cidBuilder.
func validateCID(key_value testCid) {
	cidBuilder := cid.V1Builder{
		Codec:    uint64(multicodec.DagPb),
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: -1,
	}

	c, err := cidBuilder.Sum(key_value.data)
	if err != nil {
		// If this fails to parse a buffer the input is invalid.
		panic(err)
	}
	if key_value.cid != c {
		panic("CIDs don't match")
	}
	fmt.Println("Valid CID")
}
