package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// topicPrefix namespaces all Helix GossipSub topics so they do not collide with
// other libp2p applications sharing a network.
const topicPrefix = "helix/cell/"

// TopicName builds the canonical GossipSub topic name for a Helix cell event
// stream. The cell argument is trimmed and lower-cased; an empty cell maps to
// the reserved "default" cell. This is pure logic and is unit-tested.
func TopicName(cell string) string {
	c := strings.ToLower(strings.TrimSpace(cell))
	if c == "" {
		c = "default"
	}
	return topicPrefix + c
}

// Message is the wire envelope published on a Helix topic. It carries the
// originating peer, a monotonic-ish timestamp, and an opaque payload.
type Message struct {
	From    string `json:"from"`
	Seq     uint64 `json:"seq"`
	TSUnixN int64  `json:"ts_unix_nano"`
	Payload []byte `json:"payload"`
}

// EncodeMessage serialises a Message to bytes for publishing. Pure logic;
// unit-tested round-trip against DecodeMessage.
func EncodeMessage(m Message) ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("p2p: encode message: %w", err)
	}
	return b, nil
}

// DecodeMessage parses bytes produced by EncodeMessage back into a Message.
func DecodeMessage(b []byte) (Message, error) {
	var m Message
	if err := json.Unmarshal(b, &m); err != nil {
		return Message{}, fmt.Errorf("p2p: decode message: %w", err)
	}
	return m, nil
}

// Subscription wraps a pubsub subscription with a typed receive channel and the
// raw libp2p handles needed for clean teardown.
type Subscription struct {
	Topic  string
	C      <-chan ReceivedMessage
	cancel context.CancelFunc
	sub    *pubsub.Subscription
	topicH *pubsub.Topic
}

// ReceivedMessage couples a decoded Message with the libp2p peer that delivered
// it (the propagation source) and the original sender ID.
type ReceivedMessage struct {
	Msg          Message
	ReceivedFrom peer.ID
	Err          error
}

// Subscribe joins the GossipSub topic for the given cell and returns a
// Subscription whose channel yields decoded messages until the subscription is
// cancelled or the node is closed.
func (n *Node) Subscribe(ctx context.Context, cell string) (*Subscription, error) {
	name := TopicName(cell)

	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return nil, fmt.Errorf("p2p: subscribe on closed node")
	}
	th, ok := n.topics[name]
	if !ok {
		joined, err := n.ps.Join(name)
		if err != nil {
			n.mu.Unlock()
			return nil, fmt.Errorf("p2p: join topic %q: %w", name, err)
		}
		th = joined
		n.topics[name] = th
	}
	n.mu.Unlock()

	sub, err := th.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("p2p: subscribe topic %q: %w", name, err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	ch := make(chan ReceivedMessage, 64)
	s := &Subscription{Topic: name, C: ch, cancel: cancel, sub: sub, topicH: th}

	go func() {
		defer close(ch)
		for {
			psm, err := sub.Next(subCtx)
			if err != nil {
				// Context cancelled or subscription closed: end quietly.
				return
			}
			// Ignore messages we published ourselves at the app layer so
			// callers see only remote deliveries (matches cross-cell semantics).
			if psm.ReceivedFrom == n.host.ID() {
				continue
			}
			m, derr := DecodeMessage(psm.Data)
			rm := ReceivedMessage{Msg: m, ReceivedFrom: psm.ReceivedFrom, Err: derr}
			n.log.Logf("pubsub received topic=%q from=%s seq=%d bytes=%d decode_err=%v",
				name, psm.ReceivedFrom, m.Seq, len(psm.Data), derr)
			select {
			case ch <- rm:
			case <-subCtx.Done():
				return
			}
		}
	}()

	n.mu.Lock()
	n.subs[name] = s
	n.mu.Unlock()

	n.log.Logf("pubsub subscribed topic=%q id=%s", name, n.host.ID())
	return s, nil
}

// Publish encodes and publishes payload on the cell's topic. The node joins the
// topic on demand if it has not already.
func (n *Node) Publish(ctx context.Context, cell string, seq uint64, payload []byte) error {
	name := TopicName(cell)

	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return fmt.Errorf("p2p: publish on closed node")
	}
	th, ok := n.topics[name]
	if !ok {
		joined, err := n.ps.Join(name)
		if err != nil {
			n.mu.Unlock()
			return fmt.Errorf("p2p: join topic %q: %w", name, err)
		}
		th = joined
		n.topics[name] = th
	}
	n.mu.Unlock()

	msg := Message{
		From:    n.host.ID().String(),
		Seq:     seq,
		TSUnixN: time.Now().UnixNano(),
		Payload: payload,
	}
	b, err := EncodeMessage(msg)
	if err != nil {
		return err
	}
	if err := th.Publish(ctx, b); err != nil {
		return fmt.Errorf("p2p: publish topic %q: %w", name, err)
	}
	n.log.Logf("pubsub published topic=%q from=%s seq=%d bytes=%d",
		name, n.host.ID(), seq, len(b))
	return nil
}

// Close cancels the subscription and stops its pump goroutine.
func (s *Subscription) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.sub != nil {
		s.sub.Cancel()
	}
}
