package infchat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
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
	go n.AnnounceChannel(descr)
	go n.RejoinChannel(descr)
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

	// We are no longer interested in the connection to this peer
	// ... unless it is a member of another channel we are part of.
	for _, p := range topic.ListPeers() {
		n.Host.ConnManager().Unprotect(p, descr)
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

		go func() {
			if len(topic.ListPeers()) == 0 {
				n.Cfg.Log.Printf("No connected peers for channel, message will be queued and may be dropped")
			}

			if err := topic.Publish(n.nodeContext, []byte(msg),
				pubsub.WithReadiness(pubsub.MinTopicSize(1)),
			); err != nil {
				n.Cfg.Log.Printf("Publish failed: %v", err)
			}
		}()
	case strings.HasPrefix(descriptor, DMPrefix):
		return errors.New("not implemented yet")
	default:
		return errors.New("unknown descriptor type")
	}

	return nil
}

func (n *Node) AnnounceChannel(desc string) error {
	_, err := n.Discover.Advertise(n.nodeContext, desc,
		discovery.TTL(10*time.Minute))
	return err
}

func (n *Node) RejoinChannel(desc string) error {
	pis, err := n.Discover.FindPeers(n.nodeContext, desc, discovery.Limit(100))
	if err != nil {
		return fmt.Errorf("join: find peers %s: %w", desc, err)
	}

	// The whole thing should not take more than a minute.
	ctx, cancel := context.WithTimeout(n.nodeContext, time.Minute)
	defer cancel()

	protectedCount := 0

	for peer := range pis {
		if string(peer.ID) == string(n.Host.ID()) {
			continue
		}
		if err := n.Host.Connect(ctx, peer); err != nil {
			// Not much we can do...
			continue
		}

		// We do not want 100 protected connections for each channel.
		// Even one connection should be enough to keep us part of the mesh
		// but play it safe and keep 10 connections.
		if protectedCount >= 10 {
			continue
		}

		// Prevent connection manager from closing the connection we need.
		// Using channel descriptor as protection tag as we can have protected
		// connection as long as the peer is a member of any channels.
		n.Host.ConnManager().Protect(peer.ID, desc)
		protectedCount++
	}

	// Having zero means we got no connections at all.
	if protectedCount == 0 {
		return errors.New("join: failed to connect to any peers for the channel")
	}

	return nil
}

func (n *Node) RejoinAll() error {
	// Avoid holding the lock for too long.
	n.pubsubLock.Lock()
	topicList := make([]string, 0, len(n.topics))
	for topic := range n.topics {
		topicList = append(topicList, topic)
	}
	n.pubsubLock.Unlock()

	for _, topic := range topicList {
		n.RejoinChannel(topic)
	}

	return nil
}

func (n *Node) AnnounceAll() error {
	// Avoid holding the lock for too long.
	n.pubsubLock.Lock()
	topicList := make([]string, 0, len(n.topics))
	for topic := range n.topics {
		topicList = append(topicList, topic)
	}
	n.pubsubLock.Unlock()

	for _, topic := range topicList {
		n.AnnounceChannel(topic)
	}

	return nil
}

func (n *Node) rejoinGoroutine() {
	t := time.NewTicker(n.Cfg.RejoinInterval)
	for range t.C {
		if err := n.RejoinAll(); err != nil {
			if errors.Is(err, context.Canceled) {
				t.Stop()
				return
			}
			n.Cfg.Log.Printf("rejoin failed: %v", err)
		}
	}
}

func (n *Node) announceGoroutine() {
	t := time.NewTicker(n.Cfg.AnnounceInterval)
	for range t.C {
		if err := n.AnnounceAll(); err != nil {
			if errors.Is(err, context.Canceled) {
				t.Stop()
				return
			}
			n.Cfg.Log.Printf("announce failed: %v", err)
		}
	}
}
