package updater

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	crand "crypto/rand"
	"fmt"
	"log"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestHashWorks(t *testing.T) {
	var content UpdateRequestContent
	content.BinaryCid = []byte("hello")
	content.Nonce = 0

	fmt.Println(HashRequestContent(content))

	content.BinaryCid = []byte("lolol")
	fmt.Println(HashRequestContent(content))
}

func TestEndToEnd(t *testing.T) {
	assert := assert.New(t)

	// I use uint8 and not boolean to avoid writing "true" and "false" a billion times later on.
	runTestWithKeys := func(keysTrusted []uint8, keysSigning []uint8) bool {
		assert.True(len(keysTrusted) == len(keysSigning), "Wrong argument for test.")

		trustedKeyCount := 0
		for i := range keysTrusted {
			if keysTrusted[i] == 1 {
				trustedKeyCount++
			}
		}

		var updater UpdaterData
		updater.TrustedKeys = make([]PublicKey, 0, len(keysTrusted))
		updater.LatestVerifiedRequests = make([]UpdateRequest, trustedKeyCount)

		publicKeys := make([]ed25519.PublicKey, len(keysTrusted))
		privateKeys := make([]ed25519.PrivateKey, len(keysTrusted))

		for i := range publicKeys {
			var err error
			publicKeys[i], privateKeys[i], err = ed25519.GenerateKey(crand.Reader)

			if keysTrusted[i] == 1 {
				updater.TrustedKeys = append(updater.TrustedKeys, PublicKey(publicKeys[i]))
			}

			if err != nil {
				log.Panicln(err)
			}

			// This is a dummy CID. It points to some random data cached locally basically.
			c, err := cid.Parse("QmTytvFFrGE69EWw7f3bbJhzUb3F7WTsNcPnFRH6xULUJW")
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			var req UpdateRequest

			{ // Make dummy request.

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
			if keysSigning[i] == 1 {
				result := updater.VerifyRequest(req)

				if result {
					assert.True(keysTrusted[i] == 1, "Request verified with untrusted key!")
				} else {
					assert.True(keysTrusted[i] != 1, "Request unverified with trusted key!")
				}
			}
		}

		// TODO: Fix this test!
		// "/ip4/10.0.17.23/tcp/4001/p2p/12D3KooWPnX64ZDrYZof4cyJhAA9NK2Yxygs5C1uCS2zg5x1PbHL"

		return updater.UpdateIfAppropriate()
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

	assert.False(runTestWithKeys([]uint8{1, 1, 0}, []uint8{0, 1, 0}))
	assert.False(runTestWithKeys([]uint8{0, 1, 1}, []uint8{0, 1, 0}))
	assert.False(runTestWithKeys([]uint8{1, 0, 1}, []uint8{1, 0, 0}))

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
