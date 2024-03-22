package p2p

import "github.com/libp2p/go-libp2p/core/peer"

// PeerDiscovery is the mDNS peer discovery notify instance
type PeerDiscovery struct {
	NewPeers chan peer.AddrInfo
	// Do a peer handshake to verify a new peer's validator NFT and store that alongside a public key.
	// TODO: Add new peer handshake logic by calling verify_nft from github.com/Openmesh-Network/nft-authorise
}

// HandlePeerFound will be called when a new peer is discovered
func (d *PeerDiscovery) HandlePeerFound(p peer.AddrInfo) {
	d.NewPeers <- p
}

// NewPeerDiscovery initialise a new peer discovery instance
func NewPeerDiscovery() *PeerDiscovery {
	return &PeerDiscovery{NewPeers: make(chan peer.AddrInfo)}
}
