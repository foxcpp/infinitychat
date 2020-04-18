package infchat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

func (n *Node) JoinChannel(descr string) error {
	n.pubsubLock.Lock()
	defer n.pubsubLock.Unlock()
	var (
		topic *pubsub.Topic
		err   error
		ok    bool
	)
	if topic, ok = n.topics[descr]; !ok {
		topic, err = n.PubsubProto.Join(descr)
		if err != nil {
			return fmt.Errorf("join failed: %w", err)
		}
	}

	if _, ok := n.subs[descr]; ok {
		return nil
	}

	subscription, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("join: subscribe failed: %w", err)
	}

	n.topics[descr] = topic
	n.subs[descr] = subscription

	go n.pullMessages(subscription)
	go n.rejoin(n.nodeContext, true, descr)
	return nil
}

func (n *Node) pullMessages(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(n.nodeContext)
		if err != nil {
			if strings.HasPrefix(err.Error(), "subscription cancelled") {
				return
			}
			if err == context.Canceled {
				return
			}
			n.Cfg.Log.Printf("Pull for %s failed: %v", sub.Topic(), err)
			continue
		}

		if string(msg.GetFrom()) == string(n.ID()) {
			continue
		}

		n.messages <- Message{
			Sender:  msg.GetFrom(),
			Channel: sub.Topic(),
			Text:    string(msg.Data),
		}
	}

}

func (n *Node) LeaveChannel(descr string) error {
	n.pubsubLock.Lock()
	defer n.pubsubLock.Unlock()

	topic, ok := n.topics[descr]
	if !ok {
		return fmt.Errorf("not on the channel")
	}

	sub, ok := n.subs[descr]
	if !ok {
		return fmt.Errorf("not on the channel")
	}

	delete(n.subs, descr)
	sub.Cancel()

	delete(n.topics, descr)
	if err := topic.Close(); err != nil {
		return fmt.Errorf("failed to leave: %w", err)
	}
	return nil
}

func (n *Node) Post(descriptor, msg string) error {
	switch {
	case strings.HasPrefix(descriptor, ChanPrefix):
		n.pubsubLock.Lock()
		defer n.pubsubLock.Unlock()
		topic, ok := n.topics[descriptor]
		if !ok {
			return errors.New("not on the channel")
		}

		if err := topic.Publish(context.Background(), []byte(msg)); err != nil {
			return fmt.Errorf("publish failed: %w", err)
		}
	case strings.HasPrefix(descriptor, DMPrefix):
		return errors.New("not implemented yet")
	default:
		return errors.New("unknown descriptor type")
	}

	return nil
}

// Rejoin forces immediate reannounce of channel membership.
//
// It also actively looks for other members and attempts to establish a direct connection.
func (n *Node) Rejoin(desc string) error {
	return n.rejoin(n.nodeContext, true, desc)
}

// RejoinAll froces immediate reannounce of membership for all channels.
//
// See Rejoin.
func (n *Node) RejoinAll() error {
	return n.rejoinAll(true)
}

func (n *Node) rejoinAll(traceLog bool) error {
	// Avoid holding the lock for too long.
	n.pubsubLock.Lock()
	topicList := make([]string, 0, len(n.topics))
	for topic := range n.topics {
		topicList = append(topicList, topic)
	}
	n.pubsubLock.Unlock()

	for _, topic := range topicList {
		n.rejoin(n.nodeContext, traceLog, topic)
	}

	return nil
}

func (n *Node) rejoin(ctx context.Context, traceLog bool, desc string) error {
	n.Discover.Advertise(n.nodeContext, desc)

	pis, err := n.Discover.FindPeers(n.nodeContext, desc)
	if err != nil {
		return fmt.Errorf("rejoin: find peers %s: %w", desc, err)
	}

	go func() {
		// The whole thing should not take more than a minute.
		ctx, cancel := context.WithTimeout(n.nodeContext, time.Minute)
		defer cancel()

		counter := 0
		for peer := range pis {
			if string(peer.ID) == string(n.Host.ID()) {
				continue
			}
			if err := n.Host.Connect(ctx, peer); err == nil {
				if traceLog {
					n.Cfg.Log.Printf("Connected to %v for %s", peer, desc)
				}
				counter++
			} else if traceLog {
				n.Cfg.Log.Printf("Connect to %v for %s failed: %v", peer, desc, err)
			}
		}

		nowCount := len(n.PubsubProto.ListPeers(desc))
		if lc, ok := n.knownChannelMembers[desc]; traceLog || !ok || nowCount != lc {
			n.Cfg.Log.Printf("Connected to %d peers for %s", counter, desc)
			n.knownChannelMembers[desc] = nowCount
		}
	}()

	return nil
}

func (n *Node) rejoinGoroutine() {
	t := time.NewTicker(n.Cfg.RejoinInterval)
	for range t.C {
		if err := n.rejoinAll(false); err != nil {
			if errors.Is(err, context.Canceled) {
				t.Stop()
				return
			}
			n.Cfg.Log.Printf("reannounce failed: %v", err)
		}
	}
}
