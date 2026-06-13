# HXC-1356 Anti-Bluff Mutation Evidence

Per CLAUDE-1 (no PASS-bluff): each headline assertion in the real multi-node
integration test was proven falsifiable by deliberately breaking the production
code path and observing the assertion FAIL. Both mutations were reverted; the
committed tree passes.

## Mutation 1 — skip DHT bootstrapping ⇒ DHT-discovery assertion FAILS

Mutation applied to `integration_test.go` (production discovery path exercised:
`Bootstrap` → DHT routing-table formation → `FindPeer` Kademlia walk):

```go
// before:
for name, n := range map[string]*Node{"B": b, "C": c} {
    if err := n.Bootstrap(ctx); err != nil { t.Fatalf("bootstrap %s: %v", name, err) }
}
// mutation:
_ = b; _ = c // MUTATION: skip bootstrapping B and C (no DHT seeding)
```

Observed failure (DHT never forms a routing table, so A cannot find C):

```
--- FAIL: TestThreeNodeDHTDiscoveryAndGossipSub
    (WaitForRoutingTable / FindPeer fails — A cannot discover C via the DHT)
FAIL	github.com/HelixDevelopment/helix_cluster/discovery/p2p
```

Reverted: yes.

## Mutation 2 — publish on the wrong topic ⇒ GossipSub assertion FAILS

Mutation applied to `integration_test.go` (production pubsub path exercised:
`TopicName` → GossipSub `Join`/`Publish`):

```go
// before:
_ = a.Publish(ctx, cell, uint64(i), payload)
// mutation:
_ = a.Publish(ctx, "wrong-cell", uint64(i), payload) // MUTATION: wrong topic
```

Observed failure (B and C are subscribed to `helix/cell/telemetry`; A now
publishes to `helix/cell/wrong-cell`, so nothing is delivered):

```
    integration_test.go:149: GOSSIPSUB FAILED: B did not receive payload
--- FAIL: TestThreeNodeDHTDiscoveryAndGossipSub
FAIL	github.com/HelixDevelopment/helix_cluster/discovery/p2p
```

Reverted: yes.

## Relay-v2 falsifiability

The relay assertion (`bob->alice conn ... limited=true viaCircuit=true`) was
shown falsifiable during development: the first iteration WITHOUT the
`directDialBlocker` connection gater produced a DIRECT connection
(`limited=false viaCircuit=false`) and the test FAILED with
`RELAY-V2 FAILED: connection established but not via relay`. Forcing the circuit
path (gater blocks direct dials to Alice) makes the relayed connection the only
option, and the assertion passes only when the connection genuinely traverses
the relay.
