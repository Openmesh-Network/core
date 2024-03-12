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
	"os/signal"
	"syscall"

	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/multiformats/go-multiaddr"

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

func NewInstance(trustedKeys []PublicKey) *UpdaterInstance {
	updater := UpdaterInstance{}
	updater.TrustedKeys = trustedKeys
	updater.LatestVerifiedRequests = make([]UpdateRequest, len(updater.TrustedKeys))

	return &updater
}

func (u *UpdaterInstance) VerifyRequest(req UpdateRequest) bool {
	trustedIndex := -1
	for i, tk := range u.TrustedKeys {
		if bytes.Equal(tk[:], req.PublicKey[:]) {
			trustedIndex = i
			break
		}
	}

	fmt.Println("Trusted index: ", trustedIndex)

	if trustedIndex >= 0 {
		// Verify hash and signature independently.
		hashBytes := HashRequestContent(req.Content)
		verified := ed25519.Verify(req.PublicKey[:], hashBytes, req.Signature[:])

		if verified {
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
				fmt.Println("Outdated message.")
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

			// XXX: Make 100% sure this logic makes sense.
			over51PercentThreshold := len(u.TrustedKeys)-highestTally < len(u.TrustedKeys)/2

			if over51PercentThreshold {
				isVerified = true

				var err error
				c, err = cid.Cast(u.LatestVerifiedRequests[mostPopularCidIndex].Content.BinaryCid)

				if err != nil {
					fmt.Println(err)
				} else {
					// We don't trust valid signatures with invalid CIDs.
					// Though this should never run in practice since we check this earlier.
					isVerified = false
				}
			} else {
				// No one has a majority.
				fmt.Println("No CID has a majority of keys.")
				isVerified = false
			}
		} else {
			isVerified = false
		}
	}

	if !isVerified {
		fmt.Println("Not verified didn't reach consensus.")
		return false
	}

	fmt.Println("Downloading cid...")
	{ // Download the cid.

		/* TODO: need to integrate this with existing networking code in core maybe?
		   might be better to leave this decoupled actually.
		   Problem with doing it this way is that it won't benefit from whatever routing we end up doing.
		*/

		{ // Silly ipfs stuff.

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			getFile := func(h host.Host, c cid.Cid, targetPeer string) ([]byte, error) {
				n := bsnet.NewFromIpfsHost(h, routinghelpers.Null{})
				bswap := bsclient.New(ctx, n, blockstore.NewBlockstore(datastore.NewNullDatastore()))
				n.Start(bswap)
				defer bswap.Close()

				maddr, err := multiaddr.NewMultiaddr(targetPeer)
				if err != nil {
					return nil, err
				}

				info, err := peer.AddrInfoFromP2pAddr(maddr)
				if err != nil {
					return nil, err
				}

				if err := h.Connect(ctx, *info); err != nil {
					return nil, err
				}

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

			// HACK: Need to specify this multiaddr because I need to connect so someone who is definitely seeding this CID.
			buf, err := getFile(h, c, "/ip4/10.0.17.23/tcp/4001/p2p/12D3KooWPnX64ZDrYZof4cyJhAA9NK2Yxygs5C1uCS2zg5x1PbHL")

			if err != nil {
				fmt.Println(err)
				return false
			} else {
				fmt.Println("Everything has passed")

				// XXX: think about more sensible permissions maybe
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
}

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
	}

	return h
}

func HostToString(h host.Host) string {
	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", h.ID().String()))

	addr := h.Addrs()[0]
	return addr.Encapsulate(hostAddr).String()
}

func referenceUsageDeleteLater() {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-sigs
		cancel()
	}()

	h := NewHost()
	fmt.Println("Listening on address: ", HostToString(h))

	defer cancel()

	// TODO: This needs to get the file from an actual IPFS network
	pubsub, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}

	fmt.Println("Joined topic!")
	topic, err := pubsub.Join("openmesh-core-update")
	if err != nil {
		panic(err)
	}

	fmt.Println("Subscribed to topic")
	subscription, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	var updater UpdaterInstance
	trusted_keys_base64 := []string{"JZlpAGC7aYXIupMUQN48daT/tYRulWiOC0sXFNEXFNE", "+8rZEcO928jPGlkn0CZKbXxi11twmZbj9KxxBvTa15Q"}
	updater.TrustedKeys = make([]PublicKey, len(trusted_keys_base64))
	updater.LatestVerifiedRequests = make([]UpdateRequest, len(trusted_keys_base64))

	for key_index := range trusted_keys_base64 {
		key_bytes, err := base64.RawStdEncoding.DecodeString(trusted_keys_base64[key_index])
		fmt.Println(key_bytes)

		if err != nil {
			panic(err)
		}

		for i := range key_bytes {
			updater.TrustedKeys[key_index][i] = key_bytes[i]
		}
	}

	// Check peers and log.
	// go func() {
	// 	ticker := time.NewTicker(time.Millisecond * 100)
	// 	peer_count_last := h.Peerstore().Peers().Len()
	// 	for {
	// 		select {
	// 		case <-ctx.Done():
	// 			break
	// 		case <-ticker.C:
	// 			peer_count := h.Peerstore().Peers().Len()
	// 			if peer_count_last < peer_count {
	// 				fmt.Println("Changed peers had:", peer_count_last, "now:", peer_count)
	// 				peer_count_last = peer_count
	// 			}
	// 		}
	// 	}
	// }()

	for {
		select {
		case <-ctx.Done():
			break
		default:
			fmt.Println("Waiting for message.")
			message, err := subscription.Next(ctx)
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

			updater.VerifyRequest(request)
			if updater.UpdateIfAppropriate(h) {
				fmt.Println("Success, spawned child process! Updater is finished.")
				cancel()
				os.Exit(0)
			}
		}
	}
}
