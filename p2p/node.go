// Package p2p implements a libp2p-based application-layer peer discovery and
// messaging fabric for Helix Cluster (registry item HXC-1356).
//
// It complements — and does NOT replace — the WireGuard data plane. WireGuard
// gives Helix an encrypted L3 mesh between nodes; this package gives the
// application layer a self-organising overlay on top of (or beside) it:
//
//   - a Kademlia DHT for registry-free peer discovery, and
//   - GossipSub for cross-cell event streaming.
//
// The Node type wraps a libp2p host plus a DHT and a GossipSub router, and
// exposes the small surface Helix services actually need: Bootstrap, Connect,
// Subscribe/Publish on a topic, DHT-based Advertise/FindPeers, and a clean
// Close.
package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"
)

// Config controls how a Node is constructed. The zero value is usable: it
// produces a node that listens on an OS-assigned loopback TCP port with the DHT
// in server mode and no bootstrap peers.
type Config struct {
	// ListenAddrs are the multiaddrs to listen on. When empty, the node
	// listens on a random loopback TCP port (good for in-process tests).
	ListenAddrs []string

	// BootstrapPeers are peers used to seed the DHT routing table. They are
	// dialled during Bootstrap.
	BootstrapPeers []peer.AddrInfo

	// DHTMode selects the Kademlia DHT mode. Defaults to ModeServer so that
	// in-process test networks form a real routing table without needing a
	// public, reachable bootstrap server.
	DHTMode dht.ModeOpt

	// EnableRelayService makes this node offer Circuit Relay v2 service to
	// other peers (i.e. it can act as a relay hop). Independent of whether
	// the node itself uses relays to be reached.
	EnableRelayService bool

	// EnableHolePunching turns on DCUtR so relayed connections may be
	// upgraded to direct ones when the network permits.
	EnableHolePunching bool

	// StaticRelays, when non-empty, configures this node to reserve a slot on
	// the given relays and advertise relay (circuit) addresses for itself, so
	// other peers can reach it THROUGH the relay.
	StaticRelays []peer.AddrInfo

	// ConnGater, when set, installs a libp2p connection gater. The relay test
	// uses one to forbid direct dials to a peer so the only viable path is a
	// circuit (relayed) connection — making "the connection really went through
	// the relay" provable on a single loopback host.
	ConnGater connmgr.ConnectionGater

	// Logger receives structured, line-oriented events (discovery, pubsub,
	// relay). It is never nil inside a Node; New installs a no-op logger when
	// the caller leaves it unset.
	Logger Logger
}

// Node is a running libp2p host with a Kademlia DHT and a GossipSub router.
type Node struct {
	host host.Host
	dht  *dht.IpfsDHT
	ps   *pubsub.PubSub
	disc *drouting.RoutingDiscovery
	log  Logger

	mu        sync.Mutex
	topics    map[string]*pubsub.Topic
	subs      map[string]*Subscription
	bootstrap []peer.AddrInfo
	closed    bool
}

// Logger is the minimal logging surface used by Node. It is satisfied by the
// StdLogger in log.go and is easy to adapt to zap/slog in production.
type Logger interface {
	Logf(format string, args ...any)
}

// New constructs and starts a Node from cfg. The returned Node is listening and
// its DHT is created, but Bootstrap must be called to seed the routing table.
func New(ctx context.Context, cfg Config) (*Node, error) {
	if cfg.Logger == nil {
		cfg.Logger = NopLogger{}
	}

	listen := cfg.ListenAddrs
	if len(listen) == 0 {
		// OS-assigned loopback TCP port — real network, deterministic host.
		listen = []string{"/ip4/127.0.0.1/tcp/0"}
	}
	maddrs := make([]multiaddr.Multiaddr, 0, len(listen))
	for _, a := range listen {
		m, err := multiaddr.NewMultiaddr(a)
		if err != nil {
			return nil, fmt.Errorf("p2p: bad listen addr %q: %w", a, err)
		}
		maddrs = append(maddrs, m)
	}

	// The DHT must be constructed inside the Routing option so the host wires
	// the content router for us; we capture the instance via the closure.
	var kadDHT *dht.IpfsDHT
	opts := []libp2p.Option{
		libp2p.ListenAddrs(maddrs...),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			d, err := dht.New(ctx, h, dht.Mode(dhtMode(cfg.DHTMode)))
			kadDHT = d
			return d, err
		}),
		libp2p.EnableNATService(),
	}
	if cfg.EnableRelayService {
		opts = append(opts, libp2p.EnableRelayService())
	}
	if cfg.EnableHolePunching {
		opts = append(opts, libp2p.EnableHolePunching())
	}
	if len(cfg.StaticRelays) > 0 {
		opts = append(opts,
			libp2p.EnableAutoRelayWithStaticRelays(cfg.StaticRelays),
		)
	}
	if cfg.ConnGater != nil {
		opts = append(opts, libp2p.ConnectionGater(cfg.ConnGater))
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("p2p: new host: %w", err)
	}
	if kadDHT == nil {
		_ = h.Close()
		return nil, errors.New("p2p: DHT was not constructed by routing option")
	}

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		_ = kadDHT.Close()
		_ = h.Close()
		return nil, fmt.Errorf("p2p: new gossipsub: %w", err)
	}

	n := &Node{
		host:      h,
		dht:       kadDHT,
		ps:        ps,
		disc:      drouting.NewRoutingDiscovery(kadDHT),
		log:       cfg.Logger,
		topics:    make(map[string]*pubsub.Topic),
		subs:      make(map[string]*Subscription),
		bootstrap: cfg.BootstrapPeers,
	}
	n.log.Logf("node started id=%s addrs=%v relay_service=%v holepunch=%v",
		h.ID(), n.Addrs(), cfg.EnableRelayService, cfg.EnableHolePunching)
	return n, nil
}

func dhtMode(m dht.ModeOpt) dht.ModeOpt {
	if m == 0 {
		return dht.ModeServer
	}
	return m
}

// ID returns this node's libp2p peer ID.
func (n *Node) ID() peer.ID { return n.host.ID() }

// Host exposes the underlying libp2p host for advanced callers (e.g. tests
// asserting on the peerstore or connectedness).
func (n *Node) Host() host.Host { return n.host }

// DHT exposes the underlying Kademlia DHT.
func (n *Node) DHT() *dht.IpfsDHT { return n.dht }

// Addrs returns this node's full set of dialable multiaddrs (host addr +
// peer-id component), suitable for handing to another node's Connect.
func (n *Node) Addrs() []multiaddr.Multiaddr {
	info := peer.AddrInfo{ID: n.host.ID(), Addrs: n.host.Addrs()}
	full, err := peer.AddrInfoToP2pAddrs(&info)
	if err != nil {
		return n.host.Addrs()
	}
	return full
}

// AddrInfo returns this node's peer.AddrInfo (ID + listen addrs).
func (n *Node) AddrInfo() peer.AddrInfo {
	return peer.AddrInfo{ID: n.host.ID(), Addrs: n.host.Addrs()}
}

// Connect dials a peer directly by AddrInfo. This is the explicit, non-DHT
// path used to seed a network (e.g. B and C dialling the bootstrap node A).
func (n *Node) Connect(ctx context.Context, info peer.AddrInfo) error {
	if err := n.host.Connect(ctx, info); err != nil {
		return fmt.Errorf("p2p: connect %s: %w", info.ID, err)
	}
	n.log.Logf("connected to peer=%s", info.ID)
	return nil
}

// Bootstrap seeds the DHT: it dials any configured bootstrap peers and runs the
// DHT bootstrap process so the routing table fills. It is safe to call once the
// node has at least one connected peer.
func (n *Node) Bootstrap(ctx context.Context) error {
	for _, p := range n.bootstrapPeers() {
		if p.ID == n.host.ID() {
			continue
		}
		if err := n.host.Connect(ctx, p); err != nil {
			n.log.Logf("bootstrap dial failed peer=%s err=%v", p.ID, err)
			continue
		}
		n.log.Logf("bootstrap dialed peer=%s", p.ID)
	}
	if err := n.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("p2p: dht bootstrap: %w", err)
	}
	n.log.Logf("dht bootstrap initiated id=%s", n.host.ID())
	return nil
}

// bootstrapPeers returns the configured bootstrap peers.
func (n *Node) bootstrapPeers() []peer.AddrInfo {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.bootstrap
}
