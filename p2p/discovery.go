package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

// Advertise announces this node under a rendezvous string in the DHT, so that
// other nodes calling FindPeers with the same string can discover it WITHOUT a
// central registry. This is the registry-free discovery primitive.
func (n *Node) Advertise(ctx context.Context, rendezvous string) {
	dutil.Advertise(ctx, n.disc, rendezvous)
	n.log.Logf("dht advertise rendezvous=%q id=%s", rendezvous, n.host.ID())
}

// FindPeers discovers peers advertising the given rendezvous string via the
// DHT. It returns up to limit distinct peers (excluding self), blocking until
// that many are found or ctx is done. The peers are learned purely through the
// Kademlia DHT, not from any out-of-band address feed.
func (n *Node) FindPeers(ctx context.Context, rendezvous string, limit int) ([]peer.AddrInfo, error) {
	ch, err := n.disc.FindPeers(ctx, rendezvous)
	if err != nil {
		return nil, fmt.Errorf("p2p: find peers %q: %w", rendezvous, err)
	}
	seen := make(map[peer.ID]struct{})
	out := make([]peer.AddrInfo, 0, limit)
	for {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case ai, ok := <-ch:
			if !ok {
				return out, nil
			}
			if ai.ID == n.host.ID() || ai.ID == "" {
				continue
			}
			if _, dup := seen[ai.ID]; dup {
				continue
			}
			seen[ai.ID] = struct{}{}
			out = append(out, ai)
			n.log.Logf("dht discovered peer=%s addrs=%v via rendezvous=%q",
				ai.ID, ai.Addrs, rendezvous)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}
}

// FindPeer resolves a known peer ID to its addresses purely via the DHT's
// routing layer (Kademlia walk). The caller must NOT have pre-fed the target's
// addresses; a successful result proves the DHT located the peer.
func (n *Node) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	ai, err := n.dht.FindPeer(ctx, id)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("p2p: dht findpeer %s: %w", id, err)
	}
	n.log.Logf("dht findpeer resolved peer=%s addrs=%v", ai.ID, ai.Addrs)
	return ai, nil
}

// WaitForRoutingTable blocks until the DHT routing table holds at least min
// peers or the timeout elapses. Useful in tests to avoid racing the async
// bootstrap.
func (n *Node) WaitForRoutingTable(ctx context.Context, min int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if n.dht.RoutingTable().Size() >= min {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("p2p: routing table did not reach %d peers (have %d)",
				min, n.dht.RoutingTable().Size())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
