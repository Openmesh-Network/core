package updater

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	crand "crypto/rand"
	"fmt"
	"log"
	"os/exec"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestHashWorks(t *testing.T) {
	var content UpdateRequestContent
	content.BinaryCid = []byte("hello")
	content.Nonce = 0

	// fmt.Println(HashRequestContent(content))

	content.BinaryCid = []byte("lolol")
	// fmt.Println(HashRequestContent(content))
}

func TestEndToEnd(t *testing.T) {
	assert := assert.New(t)

	// Run an ipfs daemon.
	ipfsCmd := exec.Command("ipfs", "daemon")
	go ipfsCmd.Run()

	// Get the ipfs daemon's libp2p multiaddr. This is all to simplify testing.
	ipfsId := ""
	go func() {
		for {
			// XXX: Won't work on windows, it also needs jq program.

			// Runs the ipfs id command and uses json parse program to get the first address from the address array.
			ipfsIdGetCmd := exec.Command("bash", "-c", "ipfs id | jq -r '.Addresses[0]'")
			ipfsIdBytes, err := ipfsIdGetCmd.Output()
			if err != nil {
				panic(err)
			}

			if len(ipfsIdBytes) > 0 {
				// If the id isn't empty, this means the daemon is actually active, so add the test-script to its storage.
				out, err := exec.Command("ipfs", "add", "test-script.sh").Output()
				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println(string(out))
				}
				ipfsId = string(ipfsIdBytes[:len(ipfsIdBytes)-1])

				break
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// I use uint8 and not boolean to avoid writing "true" and "false" a billion times later on.
	// keyIsTrusted are the stores whether the key is trusted or not, and keysIsMakingARequest stores whether the key will make a request.
	runTestWithKeys := func(keyIsTrusted []uint8, keyIsMakingARequest []uint8) bool {
		assert.True(len(keyIsTrusted) == len(keyIsMakingARequest), "Wrong argument for test.")

		trustedKeyCount := 0
		for i := range keyIsTrusted {
			if keyIsTrusted[i] == 1 {
				trustedKeyCount++
			}
		}

		var updater UpdaterInstance
		updater.TrustedKeys = make([]PublicKey, 0, len(keyIsTrusted))
		updater.LatestVerifiedRequests = make([]UpdateRequest, trustedKeyCount)

		publicKeys := make([]ed25519.PublicKey, len(keyIsTrusted))
		privateKeys := make([]ed25519.PrivateKey, len(keyIsTrusted))

		for i := range publicKeys {
			var err error
			publicKeys[i], privateKeys[i], err = ed25519.GenerateKey(crand.Reader)

			if keyIsTrusted[i] == 1 {
				updater.TrustedKeys = append(updater.TrustedKeys, PublicKey(publicKeys[i]))
			}

			if err != nil {
				log.Panicln(err)
			}

			// This is the CID of the test-script.sh file.
			c, err := cid.Parse("QmTytvFFrGE69EWw7f3bbJhzUb3F7WTsNcPnFRH6xULUJW")
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			var req UpdateRequest

			{ // Build a request.

				req.PublicKey = PublicKey(publicKeys[i])
				req.Content.BinaryCid = c.Bytes()
				req.Content.Nonce = 1

				hash := HashRequestContent(req.Content)

				signature, err := privateKeys[i].Sign(crand.Reader, hash, crypto.Hash(0))

				if err != nil {
					t.Error(err)
					t.FailNow()
				}

				for i := range signature {
					req.Signature[i] = signature[i]
				}

				assert.True(bytes.Equal(signature, req.Signature[:]))
			}

			// Send the request and see what happens!
			if keyIsMakingARequest[i] == 1 {
				result := updater.VerifyRequest(req)

				if result {
					assert.True(keyIsTrusted[i] == 1, "Request verified with untrusted key!")
				} else {
					assert.True(keyIsTrusted[i] != 1, "Request unverified with trusted key!")
				}
			}
		}

		// TODO: Fix this test!
		// "/ip4/10.0.17.23/tcp/4001/p2p/12D3KooWPnX64ZDrYZof4cyJhAA9NK2Yxygs5C1uCS2zg5x1PbHL"
		host := NewHost()
		if ipfsId == "" {
			fmt.Println("Couldn't find ipfs daemon ID, waiting... This is normal; if you want to speed this up for testing run `ipfs daemon` on a spare terminal.")
		}

		for ipfsId == "" {
			time.Sleep(100 * time.Millisecond)
		}

		err := ConnectToMultiaddr(context.Background(), host, ipfsId)
		if err != nil {
			panic(err)
		}

		return updater.UpdateIfAppropriate(host)
	}

	assert.True(runTestWithKeys([]uint8{1}, []uint8{1}))
	assert.True(runTestWithKeys([]uint8{1, 1}, []uint8{1, 1}))

	assert.True(runTestWithKeys([]uint8{1, 1, 0}, []uint8{1, 1, 0}))
	assert.True(runTestWithKeys([]uint8{0, 1, 1}, []uint8{0, 1, 1}))
	assert.True(runTestWithKeys([]uint8{1, 0, 1}, []uint8{1, 0, 1}))

	// These seem like false positives, but the logic is sound. If there's one trusted key and one signature then it's correct
	assert.True(runTestWithKeys([]uint8{1, 0, 0}, []uint8{1, 1, 1}))
	assert.True(runTestWithKeys([]uint8{0, 1, 0}, []uint8{1, 1, 1}))
	assert.True(runTestWithKeys([]uint8{0, 1, 0}, []uint8{1, 1, 1}))

	assert.False(runTestWithKeys([]uint8{1, 1}, []uint8{1, 0}))
	assert.False(runTestWithKeys([]uint8{0, 0}, []uint8{1, 0}))
	assert.False(runTestWithKeys([]uint8{0, 0}, []uint8{1, 1}))
	assert.False(runTestWithKeys([]uint8{0, 0}, []uint8{0, 0}))

	assert.False(runTestWithKeys([]uint8{1, 1, 0}, []uint8{0, 1, 0})) // 1 / 2
	assert.False(runTestWithKeys([]uint8{0, 1, 1}, []uint8{0, 1, 0})) // 1 / 2
	assert.False(runTestWithKeys([]uint8{1, 0, 1}, []uint8{1, 0, 0})) // 1 / 2

	assert.True(runTestWithKeys(
		[]uint8{1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]uint8{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))

	assert.False(runTestWithKeys(
		[]uint8{1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]uint8{0, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	assert.False(runTestWithKeys(
		[]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		[]uint8{1, 0, 1, 1, 1, 1, 1, 1, 1, 1}))
	assert.False(runTestWithKeys(
		[]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		[]uint8{1, 1, 1, 1, 1, 1, 1, 1, 1, 0}))
	assert.False(runTestWithKeys(
		[]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		[]uint8{1, 1, 1, 1, 1, 1, 1, 1, 1, 0}))
	assert.False(runTestWithKeys(
		[]uint8{0, 0, 0, 1, 0, 0, 0, 0, 0, 1},
		[]uint8{1, 1, 1, 1, 1, 1, 1, 1, 1, 0}))
	assert.False(runTestWithKeys(
		[]uint8{1, 0, 0, 1, 0, 0, 0, 0, 0, 1},
		[]uint8{0, 0, 1, 1, 1, 1, 1, 1, 1, 0}))
}
