# discovery/p2p — libp2p application-layer P2P (HXC-1356)

A self-contained Go module providing a libp2p-based peer-discovery and
messaging fabric for Helix Cluster. It **complements WireGuard, it does not
replace it**: WireGuard gives Helix an encrypted L3 mesh between nodes; this
layer gives the application a self-organising overlay with

- a **Kademlia DHT** for registry-free peer discovery, and
- **GossipSub** for cross-cell event streaming,
- optional **Circuit Relay v2** for reaching peers behind NAT.

Module path: `github.com/HelixDevelopment/helix_cluster/discovery/p2p`
(self-contained — its own `go.mod`; does not touch the root module/workspace).

## Surface

```go
n, err := p2p.New(ctx, p2p.Config{ /* listen, bootstrap peers, logger, ... */ })
n.Bootstrap(ctx)                       // seed the DHT routing table
n.Connect(ctx, addrInfo)               // explicit dial (network seeding)
ai, _ := n.FindPeer(ctx, peerID)       // registry-free DHT discovery (Kademlia walk)
n.Advertise(ctx, "rendezvous")         // DHT provide
peers, _ := n.FindPeers(ctx, "rv", 3)  // DHT find-providers
sub, _ := n.Subscribe(ctx, "cellA")    // GossipSub subscribe
n.Publish(ctx, "cellA", seq, payload)  // GossipSub publish
n.Close()                              // clean shutdown
```

Topic naming and message encode/decode are pure, unit-tested logic
(`TopicName`, `EncodeMessage`/`DecodeMessage`).

## Files

| File | Purpose |
|------|---------|
| `node.go` | `Node` wrapper: libp2p host + Kademlia DHT + GossipSub, `New`/`Connect`/`Bootstrap`. |
| `discovery.go` | DHT discovery: `Advertise`, `FindPeers`, `FindPeer`, `WaitForRoutingTable`. |
| `pubsub.go` | GossipSub: `TopicName`, `Message` codec, `Subscribe`/`Publish`. |
| `log.go` | `Logger` interface, `NopLogger`, capturing `StdLogger`. |
| `close.go` | Idempotent `Node.Close`. |
| `pubsub_test.go` | Fast unit tests (topic naming, codec, logger). |
| `integration_test.go` | Real multi-node tests (DHT discovery, GossipSub delivery, Circuit Relay v2). |

## Tests

Standalone (the root workspace is bypassed via a local `go.work` or `GOWORK=off`):

```bash
cd discovery/p2p
GOWORK=off go test ./... -race -v
```

- `TestThreeNodeDHTDiscoveryAndGossipSub` — 3 real libp2p hosts on loopback TCP.
  Node A learns node C's addresses **purely via the Kademlia DHT** (A is never
  told C's address; a precondition assert enforces this). A message A publishes
  on a topic is received, byte-for-byte, by **both** B and C.
- `TestCircuitRelayV2` — a real Circuit Relay v2 hop service; Alice reserves a
  slot (real reservation voucher); Bob — fitted with a connection gater that
  blocks every direct dial to Alice — connects to Alice and the connection is
  asserted to be `limited / via /p2p-circuit` (genuinely relayed).
  **DCUtR direct-upgrade is DEFERRED**: a single loopback host has no NAT to
  hole-punch through, so a genuine relayed→direct upgrade cannot be exercised
  or asserted here. The relayed connection itself is really established.

Anti-bluff falsifiability is documented in `MUTATION_EVIDENCE.md`.

## Library versions

- `github.com/libp2p/go-libp2p v0.48.0`
- `github.com/libp2p/go-libp2p-kad-dht v0.40.0`
- `github.com/libp2p/go-libp2p-pubsub v0.16.0`
- `github.com/multiformats/go-multiaddr v0.16.1`
