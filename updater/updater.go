package updater

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"os"
	"time"

	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/multiformats/go-multiaddr"
	"github.com/openmesh-network/core/networking/p2p"

	"github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/ipld/merkledag"
	unixfile "github.com/ipfs/boxo/ipld/unixfs/file"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"

	bsclient "github.com/ipfs/boxo/bitswap/client"
	bsnet "github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/files"
)

type PublicKey [ed25519.PublicKeySize]byte
type Signature [ed25519.SignatureSize]byte

type UpdateRequest struct {
	PublicKey PublicKey
	Signature Signature

	Content UpdateRequestContent
}

type UpdateRequestContent struct {
	Nonce     int64
	BinaryCid []byte
}

type UpdaterInstance struct {
	TrustedKeys []PublicKey

	LatestVerifiedRequests []UpdateRequest
	P2pInstance            *p2p.Instance
	// CurrentVersion         [32]byte // TODO: this should be passed as an argv to child process
}

func HashRequestContent(content UpdateRequestContent) []byte {
	h := sha256.New()
	e := gob.NewEncoder(h)
	e.Encode(content)
	return h.Sum(nil)
}

var numbers = []int{
	1,
	plusTwo(2),
}

func plusTwo(a int) int {
	return 2 + a
}

func (u *UpdaterInstance) testFunc() {
	fmt.Println("I love this!")
}

func PublicKeyFromBase64(base64KeyString string) PublicKey {
	key_bytes, err := base64.RawStdEncoding.DecodeString(base64KeyString)

	if err != nil {
		panic(err)
	}

	publicKey := PublicKey{}

	for i := range key_bytes {
		publicKey[i] = key_bytes[i]
	}

	return publicKey
}

func NewInstance(trustedKeys []PublicKey, p2pInstance *p2p.Instance) *UpdaterInstance {
	updater := UpdaterInstance{}
	updater.TrustedKeys = trustedKeys
	updater.LatestVerifiedRequests = make([]UpdateRequest, len(updater.TrustedKeys))
	updater.P2pInstance = p2pInstance

	return &updater
}

func (u *UpdaterInstance) VerifyRequest(req UpdateRequest) bool {
	// This matches the request's key to the list of public keys.
	trustedIndex := -1
	for i, tk := range u.TrustedKeys {
		if bytes.Equal(tk[:], req.PublicKey[:]) {
			trustedIndex = i
			break
		}
	}

	// NOTE: Harry maybe uncomment this for debugging.
	// fmt.Println("Trusted index: ", trustedIndex)

	if trustedIndex >= 0 {
		// Verify hash and signature independently.
		hashBytes := HashRequestContent(req.Content)
		verified := ed25519.Verify(req.PublicKey[:], hashBytes, req.Signature[:])

		if verified {
			// NOTE: Harry maybe uncomment this for debugging.
			// fmt.Println("Trusted index: ", trustedIndex)
			// fmt.Println("LatestVerifiedRequests: ", u.LatestVerifiedRequests)

			// Verified requests might be empty.
			if u.LatestVerifiedRequests[trustedIndex].Content.Nonce < req.Content.Nonce {
				// This means this verified message is newer so it should take the place of the old one in the queue.
				u.LatestVerifiedRequests[trustedIndex] = req
				return true
			} else {
				// NOT VERIFIED
				// TODO:Handle case where outdated message is still being received
				fmt.Println("Outdated message. Nonce local ", u.LatestVerifiedRequests[trustedIndex].Content.Nonce, req.Content.Nonce)
				return false
			}
		} else {
			// NOT VERIFIED
			// TODO:Handle case where public key is right but signature is wrong
			fmt.Println("Public key is right, signature incorrect.")
			return false
		}
	} else {
		// NOT VERIFIED
		// TODO: Handle case where someone sent a message with unknown receiver
		fmt.Println("Message with unknown receiver.")
		return false
	}
}

// TODO: Move this somewhere more appropriate
func ConnectToMultiaddr(ctx context.Context, h host.Host, targetPeer string) error {
	maddr, err := multiaddr.NewMultiaddr(targetPeer)
	if err != nil {
		return err
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return err
	}

	err = h.Connect(ctx, *info)
	if err != nil {
		return err
	}

	return nil
}

func (u *UpdaterInstance) UpdateIfAppropriate(h host.Host) bool {
	isVerified := false
	c := cid.Cid{}

	{ // Check if I should update.
		// RULE: the majority of the trusted keys backup a new version number.

		cidIndices := []int32{}
		{ // Gives a list of all unique CIDs.
			for ci, r := range u.LatestVerifiedRequests {
				// Nonce 0 is not a valid request.
				if r.Content.Nonce > 0 {
					// Only proceed if the CID is valid!
					_, parseError := cid.Parse(r.Content.BinaryCid)
					if parseError == nil {
						cidIndex := int32(ci)

						unique := true
						for _, i := range cidIndices {
							// Again, nonce 0 is invalid.
							if u.LatestVerifiedRequests[i].Content.Nonce > 0 {
								if cidIndex != i && bytes.Equal(r.Content.BinaryCid, u.LatestVerifiedRequests[i].Content.BinaryCid) {
									unique = false
								}
							}
						}

						if unique {
							cidIndices = append(cidIndices, cidIndex)
						}
					}
				}
			}
		}

		fmt.Println("Cid indices", cidIndices)
		fmt.Println("Latest verified requests", len(u.LatestVerifiedRequests))

		if len(cidIndices) > 0 {
			mostPopularCidIndex := cidIndices[0]
			highestTally := 0
			{
				for _, v := range cidIndices {
					tally := 0
					for _, r := range u.LatestVerifiedRequests {
						if bytes.Equal(r.Content.BinaryCid, u.LatestVerifiedRequests[v].Content.BinaryCid) {
							tally++
							fmt.Println("This runs twice somehow.")
						}
					}

					if tally > highestTally {
						mostPopularCidIndex = v
						highestTally = tally
					}
				}
			}

			// XXX: Make 100% sure this logic makes sense:
			// Want #ValidSignatures >= #TotalKeys * 2/3 to be true for 66% concensus.
			// So it simplifies to 3 * #ValidSignatures >= 2 * #TotalKeys.
			over66PercentThreshold := len(u.TrustedKeys)*2 <= highestTally*3
			if over66PercentThreshold {
				var err error
				c, err = cid.Cast(u.LatestVerifiedRequests[mostPopularCidIndex].Content.BinaryCid)

				if err != nil {
					// We don't trust valid signatures with invalid CIDs.
					// Though this should never run in practice since we check this earlier.
					isVerified = false
					fmt.Println("Invalid CID.")
					fmt.Println(err)
				} else {
					isVerified = true
				}
			} else {
				// No one has a majority.
				fmt.Println("No CID has a majority of keys.")
				isVerified = false
			}
		} else {
			fmt.Println("No CIDs.")
			isVerified = false
		}
	}

	if !isVerified {
		fmt.Println("Not verified didn't reach consensus.")
		return false
	}

	fmt.Println("Downloading cid...")
	{ // Download the cid.
		// TODO: Maybe take ctx as a parameter.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		getFile := func(h host.Host, c cid.Cid) ([]byte, error) {
			n := bsnet.NewFromIpfsHost(h, routinghelpers.Null{})
			bswap := bsclient.New(ctx, n, blockstore.NewBlockstore(datastore.NewNullDatastore()))
			n.Start(bswap)
			defer bswap.Close()

			// I could write a novel on how bad this API is. But whatever, we have to disconnect
			// and reconnect to our peers if we want them to be actually "registered" by bsnet.
			for _, peerId := range h.Network().Peers() {
				// XXX: Find a good timeout for reconnecting to old peers. Also find if there's a better way to handle this bitswap issue.
				ctx, cancel := context.WithTimeout(ctx, time.Second*10)
				defer cancel()

				fmt.Println("Connecting to:", peerId)
				err := n.DisconnectFrom(ctx, peerId)
				fmt.Println("Success:", peerId)
				if err != nil {
					fmt.Println("Failed to disconnect from:", peerId)
					// panic(err)
				} else {
					err = n.ConnectTo(ctx, peerId)
					fmt.Println("Success:", peerId)
					if err != nil {
						fmt.Println("Failed to connect to:", peerId)
						//panic(err)
					}
				}
			}
			time.Sleep(time.Second * 10)

			dserv := merkledag.NewReadOnlyDagService(merkledag.NewSession(ctx, merkledag.NewDAGService(blockservice.New(blockstore.NewBlockstore(datastore.NewNullDatastore()), bswap))))
			nd, err := dserv.Get(ctx, c)
			if err != nil {
				return nil, err
			}

			unixFSNode, err := unixfile.NewUnixfsFile(ctx, dserv, nd)
			if err != nil {
				return nil, err
			}

			var buf bytes.Buffer
			if f, ok := unixFSNode.(files.File); ok {
				if _, err := io.Copy(&buf, f); err != nil {
					return nil, err
				}
			}
			return buf.Bytes(), nil
		}

		// ConnectToMultiaddr(ctx, h, "/ip4/10.0.17.23/tcp/4001/p2p/12D3KooWPawRnPFja1wdPs579nQyimdTtpednBR7bG6aS5kbh7xB")
		buf, err := getFile(h, c)

		if err != nil {
			fmt.Println(err)
			return false
		} else {
			fmt.Println("Everything has passed")

			// XXX: Think about more sensible permissions maybe. Shouldn't matter if it's running in a container 🤷.
			os.WriteFile("executable-file", buf, 0o777)

			var attr os.ProcAttr
			_, err := os.StartProcess("executable-file", []string{"executable-file", "wazzup"}, &attr)

			if err != nil {
				fmt.Println("error launching new process: ", err)
				return false
			} else {
				return true
			}
		}
	}
}

// Used for debugging, will panic on err.
func NewHost() host.Host {
	var h host.Host
	{
		priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
		if err != nil {
			panic(err)
		}

		opts := []libp2p.Option{
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", "0.0.0.0", 0)),
			libp2p.Identity(priv),
		}

		h, err = libp2p.New(opts...)
		if err != nil {
			panic(err)
		}
	}

	return h
}

func HostToString(h host.Host) string {
	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", h.ID().String()))

	addr := h.Addrs()[0]
	return addr.Encapsulate(hostAddr).String()
}

func (updater *UpdaterInstance) Start(ctx context.Context) {
	fmt.Println("Updater listening on address: ", HostToString(*updater.P2pInstance.Host))

	// TODO: This needs to get the file from an actual IPFS network
	err := updater.P2pInstance.JoinTopic("openmesh-core-update")
	if err != nil {
		// HACK: Should handle this sensibly.
		panic(err)
	}

	subscription, err := updater.P2pInstance.Subscribe("openmesh-core-update")
	if err != nil {
		// HACK: Should handle this sensibly.
		panic(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				break
			case message := <-subscription:
				fmt.Println("Got message.")

				if err != nil {
					fmt.Println(err)
				}
				fmt.Println(message)

				var request_buffer bytes.Buffer
				request_buffer.Write(message.Data)
				decoder := gob.NewDecoder(&request_buffer)

				var request UpdateRequest
				decoder.Decode(&request)

				fmt.Println(message.Data)
				fmt.Println(request)

				// TODO: Handle misbehaving nodes here.
				if !updater.VerifyRequest(request) {
					// TODO: I think I can disconnect from this peer here. Maybe assign social credit score or something?
				}
				if updater.UpdateIfAppropriate(*updater.P2pInstance.Host) {
					fmt.Println("Success, spawned child process! Updater is finished.")
					os.Exit(0)
				}
			}
		}
	}()
}
