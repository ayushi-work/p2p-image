package p2p

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

const (
	// Protocol IDs for different message types
	ProtocolConsensus = "/p2p-image/consensus/1.0.0"
	ProtocolProcess   = "/p2p-image/process/1.0.0"
	ProtocolHashStore = "/p2p-image/hashstore/1.0.0"

	// mDNS service name for peer discovery
	ServiceName = "p2p-image-network"
)

// Node represents a P2P node (gateway or peer)
type Node struct {
	host   host.Host
	ctx    context.Context
	cancel context.CancelFunc

	// Peer management
	peers     map[peer.ID]peer.AddrInfo
	peerMutex sync.RWMutex

	// Node type
	isGateway bool
}

// NodeConfig contains configuration for creating a node
type NodeConfig struct {
	ListenPort     int
	IsGateway      bool
	BootstrapPeers []string // Multiaddrs of bootstrap peers
}

// NewNode creates a new P2P node
func NewNode(ctx context.Context, config NodeConfig) (*Node, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Create libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.ListenPort),
		),
		libp2p.EnableNATService(),
		libp2p.EnableRelay(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	node := &Node{
		host:      h,
		ctx:       ctx,
		cancel:    cancel,
		peers:     make(map[peer.ID]peer.AddrInfo),
		isGateway: config.IsGateway,
	}

	// Start peer discovery
	if err := node.startDiscovery(); err != nil {
		h.Close()
		cancel()
		return nil, fmt.Errorf("failed to start discovery: %w", err)
	}

	return node, nil
}

// startDiscovery initializes mDNS peer discovery
func (n *Node) startDiscovery() error {
	notifee := &discoveryNotifee{node: n}

	_ = mdns.NewMdnsService(n.host, ServiceName, notifee)
	// Note: mdns service runs automatically, no need to store or start

	return nil
}

// discoveryNotifee handles peer discovery events
type discoveryNotifee struct {
	node *Node
}

func (d *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Don't add ourselves
	if pi.ID == d.node.host.ID() {
		return
	}

	fmt.Printf("🔍 mDNS discovered peer: %s\n", pi.ID)

	d.node.peerMutex.Lock()
	d.node.peers[pi.ID] = pi
	d.node.peerMutex.Unlock()

	// Connect to the peer
	if err := d.node.host.Connect(d.node.ctx, pi); err != nil {
		fmt.Printf("Failed to connect to peer %s: %v\n", pi.ID, err)
	} else {
		fmt.Printf("Connected to peer: %s\n", pi.ID)
	}
}

// GetPeers returns list of connected peers
func (n *Node) GetPeers() []peer.AddrInfo {
	n.peerMutex.RLock()
	defer n.peerMutex.RUnlock()

	peers := make([]peer.AddrInfo, 0, len(n.peers))
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	return peers
}

// GetPeerCount returns number of connected peers
func (n *Node) GetPeerCount() int {
	n.peerMutex.RLock()
	defer n.peerMutex.RUnlock()
	return len(n.peers)
}

// Host returns the underlying libp2p host
func (n *Node) Host() host.Host {
	return n.host
}

// ID returns the node's peer ID
func (n *Node) ID() peer.ID {
	return n.host.ID()
}

// IsGateway returns whether this node is a gateway
func (n *Node) IsGateway() bool {
	return n.isGateway
}

// SetStreamHandler sets a handler for a protocol
func (n *Node) SetStreamHandler(proto protocol.ID, handler func(Stream)) {
	n.host.SetStreamHandler(proto, func(s StreamImpl) {
		handler(&streamWrapper{s})
	})
}

// Close shuts down the node
func (n *Node) Close() error {
	n.cancel()
	return n.host.Close()
}

// Context returns the node's context
func (n *Node) Context() context.Context {
	return n.ctx
}
