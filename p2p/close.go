package p2p

import "errors"

// Close performs a clean, idempotent shutdown: it cancels active
// subscriptions, closes the DHT, and closes the libp2p host (which tears down
// all transports and connections).
func (n *Node) Close() error {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return nil
	}
	n.closed = true
	subs := make([]*Subscription, 0, len(n.subs))
	for _, s := range n.subs {
		subs = append(subs, s)
	}
	n.subs = nil
	n.mu.Unlock()

	for _, s := range subs {
		s.Close()
	}

	var errs []error
	if n.dht != nil {
		if err := n.dht.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if n.host != nil {
		if err := n.host.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	n.log.Logf("node closed id=%s", n.host.ID())
	return errors.Join(errs...)
}
