package infchat

import (
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

// IsJoined reports whether we are currently a member of the specified channel.
func (n *Node) IsJoined(chanDescr string) bool {
	n.pubsubLock.Lock()
	defer n.pubsubLock.Unlock()

	_, ok := n.subs[chanDescr]
	return ok
}

func (n *Node) ConnectedMembers(chanDescr string) []peer.ID {
	members := n.PubsubProto.ListPeers(chanDescr)

	res := make([]peer.ID, 0, len(members))
	for _, p := range members {
		if len(n.Host.Network().ConnsToPeer(p)) != 0 {
			res = append(res, p)
		}
	}
	return res
}

type StatusData struct {
	State string

	ConnectedPeers int
	KnownPeers     int
	PubsubTopics   int

	NAT bool
}

func (n *Node) Status() StatusData {
	s := StatusData{
		State:          "???",
		ConnectedPeers: len(n.Host.Network().Peers()),
		KnownPeers:     len(n.Host.Peerstore().Peers()),
		PubsubTopics:   len(n.PubsubProto.GetTopics()),
		NAT:            n.AutonatProto.Status() == network.ReachabilityPrivate,
	}

	noBootstrap := len(n.Cfg.Bootstrap) == 0

	if s.ConnectedPeers == 0 {
		s.State = "Alone."
	} else if s.ConnectedPeers <= len(n.Cfg.Bootstrap) && !noBootstrap {
		s.State = "Bootstrapping..."
	} else if s.ConnectedPeers < n.Cfg.ConnsLow {
		s.State = "Active."
	} else {
		s.State = "Ready."
	}

	return s
}
