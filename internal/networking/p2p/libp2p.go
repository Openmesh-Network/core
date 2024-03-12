package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
	"log"
	"openmesh.network/openmesh-core/internal/config"
	"sync"
	"time"
)

// Instance is the libp2p instance for networking usage.
type Instance struct {
	cancelCtx context.Context
	host      *host.Host     // Host for libp2p.
	DHT       *dht.IpfsDHT   // Kademlia DHT for resource locating.
	Discovery *PeerDiscovery // mDNS peer discovery instance.

	PubSub *pubsub.PubSub           // Gossip pub-sub service.
	topics map[string]*pubsub.Topic // Key: topic; Value: handle for that topic.

	nbOfPeers int        // Number of peers this node was connected to.
	peersLock sync.Mutex // To resolve synchronisation issue.

	startMDNS func() error
	closeMDNS func() error
}

// NewInstance initialises a blank libp2p instance.
func NewInstance(c context.Context) *Instance {
	return &Instance{
		cancelCtx: c,
		topics:    make(map[string]*pubsub.Topic),
	}
}

// SetP2PHost uses an existing libp2p host to initialise the instance.
func (i *Instance) SetP2PHost(existingHost *host.Host) *Instance {
	i.host = existingHost
	return i
}

// Build constructs the P2P instance using the given configuration.
func (i *Instance) Build() (*Instance, error) {
	var err error

	// Initialise a default libp2p host if not present.
	if i.host == nil {
		i.host, err = NewDefaultP2PHost()

		if err != nil {
			return i, err
		}
	}

	// Initialise Kademlia DHT instance.
	i.DHT, err = dht.New(context.Background(), *i.host, dht.Mode(dht.ModeAutoServer))
	i.DHT.Validator = NewDHTValidator()
	if err != nil {
		log.Fatalf("Failed to create Kademlia DHT: %s", err.Error())
	}

	// Initialise mDNS peer discovery
	if i.Discovery == nil {
		i.Discovery = NewPeerDiscovery()
	}
	mdnsSrv := mdns.NewMdnsService(*i.host, config.Config.P2P.GroupName, i.Discovery)
	i.startMDNS = mdnsSrv.Start
	i.closeMDNS = mdnsSrv.Close

	// Initialise Gossip pub-sub
	i.PubSub, err = pubsub.NewGossipSub(context.Background(), *i.host)
	if err != nil {
		log.Fatalf("Failed to create Gossip pub-sub service: %s", err.Error())
	}

	log.Printf("Successfully initialised a libp2p instance with ID %s", (*i.host).ID())
	return i, nil
}

// Start using mDNS to join this client to the existing cluster
func (i *Instance) Start() error {
	// Start trying to connectToNewPeer to new peers
	go i.connectToNewPeer(i.cancelCtx)
	bc, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start mDNS for peer discovery
	if err := i.startMDNS(); err != nil {
		return err
	}

	// Connect this DHT client to the DHT cluster
	if err := i.DHT.Bootstrap(bc); err != nil {
		return err
	}
	return nil
}

// Stop shutdown the libp2p instance and close this dht client
// It does not destroy the whole DHT itself
func (i *Instance) Stop() error {
	if err := i.DHT.Close(); err != nil {
		return err
	}
	if err := i.closeMDNS(); err != nil {
		return err
	}
	if err := (*i.host).Close(); err != nil {
		return err
	}

	return nil
}

// JoinTopic join this instance to the specific pub-sub topic
func (i *Instance) JoinTopic(topic string) error {
	if _, ok := i.topics[topic]; ok {
		return fmt.Errorf(`topic "%s" already exists on this instance`, topic)
	}

	topicHandle, err := i.PubSub.Join(topic)
	if err != nil {
		return err
	}

	i.topics[topic] = topicHandle
	return nil
}

// Publish a message to the specific topic
func (i *Instance) Publish(topic string, message []byte) error {
	// Check if the topic handle exists on this instance (i.e., joined this topic)
	handle, exists := i.topics[topic]
	if !exists {
		return fmt.Errorf(`topic %s does not exists on this instance`, topic)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := handle.Publish(ctx, message); err != nil {
		return err
	}
	return nil
}

// Subscribe to a specific topic
func (i *Instance) Subscribe(topic string) (<-chan *pubsub.Message, error) {
	// Check if the topic handle exists on this instance (i.e., joined this topic)
	handle, exists := i.topics[topic]
	if !exists {
		return nil, fmt.Errorf(`topic %s does not exists on this instance`, topic)
	}

	subscribe, err := handle.Subscribe()
	if err != nil {
		return nil, err
	}

	msgChan := make(chan *pubsub.Message, 100)
	go i.waitMsg(subscribe, msgChan)

	return msgChan, nil
}

// waitMsg wait for new messages and send it to the specific channel
func (i *Instance) waitMsg(handle *pubsub.Subscription, ch chan<- *pubsub.Message) {
	for {
		msg, err := handle.Next(i.cancelCtx)
		if err != nil {
			log.Printf("Failed to receve message: %s", err.Error())
			continue
		}

		// Only consider messages delivered by other peers
		if msg.ReceivedFrom == (*i.host).ID() {
			continue
		}
		// Handle it over via channel
		ch <- msg
	}
}

// connectToNewPeer try to connect to peers discovered by mDNS if peer limit not exceeded
func (i *Instance) connectToNewPeer(ctx context.Context) {
	for {
		select {
		case p := <-i.Discovery.NewPeers:
			// Check how many peers this node currently connected to
			i.peersLock.Lock()

			// Don't connect to new peers if peer limit exceeded
			if i.nbOfPeers >= config.Config.P2P.PeerLimit {
				log.Printf(
					"Peer limit %d exceeded, ignore newly discovered peer %s",
					config.Config.P2P.PeerLimit,
					p.ID,
				)
				i.peersLock.Unlock()
				continue
			}

			// Otherwise connect to this peer
			i.peersLock.Unlock()
			err := (*i.host).Connect(context.Background(), p)
			if err != nil {
				log.Printf("Failed to connect to peer %s: %s", p.ID, err.Error())
				log.Printf("Start retry to connect to peer...")
				go i.tryConnect(10, p)
				continue
			}
			i.increaseNbOfPeers()
			log.Printf("Successfully establised connection to peer %s", p.ID)
			continue
		case <-ctx.Done():
			return
		}
	}
}

// tryConnect retry connectToNewPeer to the peer discovered
func (i *Instance) tryConnect(cnt int, p peer.AddrInfo) {
	t := time.NewTicker(5 * time.Second)
	for cnt > 0 {
		select {
		case <-t.C:
			err := (*i.host).Connect(context.Background(), p)
			if err != nil {
				log.Printf("Failed to connect to peer %s: %s, retry after 5 seconds...", p.ID, err.Error())
				continue
			}

			i.increaseNbOfPeers()
			log.Printf("Successfully establised connection to peer %s", p.ID)
			return
		}
	}
	log.Printf("Retry limit exceeded, will not continue trying to connect to peer %s", p.ID)
}

// increaseNbOfPeers is a synchronisation-safe operation that increase number of peers in the instance by 1
func (i *Instance) increaseNbOfPeers() {
	i.peersLock.Lock()
	defer i.peersLock.Unlock()

	i.nbOfPeers++
	log.Printf("Number of peers discovered and connected to: %d", i.nbOfPeers)
}

// NewDefaultP2PHost initialise a new libp2p host
func NewDefaultP2PHost() (*host.Host, error) {
	// Creates a new RSA key pair for this host
	r := rand.Reader
	sk, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		log.Fatalf("Failed to create RSA key pair for host initialisation: %s", err.Error())
		return nil, err
	}

	// Create a new libp2p instance that listen to a random port
	listen, err := multiaddr.NewMultiaddr(fmt.Sprintf(
		"/ip4/%s/tcp/%d",
		config.Config.P2P.Addr,
		config.Config.P2P.Port,
	))
	if err != nil {
		log.Fatalf("Failed to create P2P instance: %s", err.Error())
		return nil, err
	}

	h, err := libp2p.New(
		libp2p.ListenAddrs(listen),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Identity(sk),
	)
	if err != nil {
		log.Fatalf("Failed to initialise a default libp2p host: %s", err.Error())
	}

	return &h, nil
}
