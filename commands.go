package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
)

var ErrInterrupt = errors.New("interrupt requested")

func handleCommand(line string) error {
	type cmd struct {
		Description string
		FullHelp    string
		Callback    func([]string)
	}

	cmds := map[string]cmd{
		"join": {
			Description: "Join a chat channel",
			FullHelp: `/join <descriptor>

Note that it might not be possible to send messages immediately, wait for the
"connected to N peers" message.`,
			Callback: joinCmd,
		},
		"leave": {
			Description: "Leave a previously joined chat channel",
			Callback:    leaveCmd,
		},
		"connect": {
			Description: "Ensure connection to a peer",
			FullHelp: `/connect <multiaddress>

Establish libp2p connection to the other node.`,
			Callback: connectCmd,
		},
		"rejoin": {
			Description: "Force reannounce of channel membership",
			FullHelp: `/rejoin

Might help to accelerate mesh reconnection in case of nodes falling offline.`,
			Callback: rejoinCmd,
		},
		"msg": {
			Description: "Send message to a specified channel",
			FullHelp: `/msg <descriptor> <message>

Channel must be joined prior using /join`,
			Callback: msgCmd,
		},
		"id": {
			Description: "Show local node ID",
			Callback: func(p []string) {
				if len(p) != 1 {
					Out.Msg("local", true, "Usage: /id")
				}
				Out.Msg("local", true, "my ID: %v", node.ID())
			},
		},
		"peers": {
			Description: "Show list of connected peers and addresses",
			Callback:    peersCmd,
		},
		"stat": {
			Description: "Display available information about objects referenced by descriptors",
			Callback:    statCmd,
		},
		"ping": {
			Description: "Measure connection latency to the peer",
			Callback:    pingCmd,
		},
		"quit": {
			Description: "Shutdown the client",
			Callback:    nil,
			// Specially handled below.
		},
	}

	cmds["help"] = cmd{
		Description: "Show available commands or extended command help (/help [command])",
		FullHelp:    `/help [command]`,
		Callback: func(parts []string) {
			switch len(parts) {
			case 1:
				Out.Msg("local", true, "available commands:")
				cmdList := make([]string, 0, len(cmds))
				maxLen := 0
				for cmd := range cmds {
					if len(cmd) > maxLen {
						maxLen = len(cmd)
					}
					cmdList = append(cmdList, cmd)
				}
				// Put into slice and sort to give it determistic ordering.
				sort.Strings(cmdList)
				for _, cmd := range cmdList {
					Out.Msg("local", true, "/%s%s%s", cmd, strings.Repeat(" ", maxLen-len(cmd)+4), cmds[cmd].Description)
				}
			case 2:
				key := strings.ToLower(strings.TrimPrefix(parts[1], "/"))
				info, ok := cmds[key]
				if !ok {
					Out.Msg("local", true, "unknown command")
					return
				}
				Out.Msg("local", true, "%s\n%s", info.Description, info.FullHelp)
			default:
				Out.Msg("local", true, "usage: /help [command]")
			}
		},
	}

	parts := strings.Split(line, " ")
	key := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	if key == "quit" {
		return ErrInterrupt
	}
	cb, ok := cmds[key]
	if !ok {
		Out.Msg("local", true, "unknown command /%s, try /help", key)
		return nil
	}
	cb.Callback(parts)

	return nil
}

func joinCmd(commandParts []string) {
	if len(commandParts) != 2 {
		Out.Msg("local", true, "usage: /join <channel descriptor>")
		return
	}
	descriptor := commandParts[1]

	if !strings.HasPrefix(descriptor, "#") {
		Out.Msg("local", true, "invalid channel descriptor")
		return
	}

	topicName := chanPrefix + descriptor[1:]

	topicsLock.Lock()
	var (
		topic *pubsub.Topic
		err   error
		ok    bool
	)
	if topic, ok = topics[topicName]; !ok {
		topic, err = pubsubInst.Join(topicName)
		if err != nil {
			Out.Msg("local", true, "join failed: %v", err)
			topicsLock.Unlock()
			return
		}
	}
	topicsLock.Unlock()

	subscription, err := topic.Subscribe()
	if err != nil {
		Out.Msg("local", true, "Subscribe failed: %v", err)
		return
	}

	topics[topicName] = topic
	subscriptions[topicName] = subscription

	go pullMessages(subscription)

	Out.Msg("local", true, "Joined %s", topicName)
	currentTopic = topicName

	discover := discovery.NewRoutingDiscovery(kDHT)
	rejoin(context.Background(), discover, true, topicName)
}

func leaveCmd(commandParts []string) {
	if len(commandParts) != 2 {
		Out.Msg("local", true, "Usage: /leave <channel descriptor>")
		return
	}
	descriptor := commandParts[1]

	if !strings.HasPrefix(descriptor, "#") {
		Out.Msg("local", true, "Invalid channel descriptor")
		return
	}

	topicName := chanPrefix + descriptor[1:]

	topicsLock.Lock()
	defer topicsLock.Unlock()
	topic, ok := topics[topicName]
	if !ok {
		Out.Msg("local", true, "Not on the channel!")
		return
	}

	subscription, ok := subscriptions[topicName]
	if !ok {
		Out.Msg("local", true, "Not on the channel!")
		return
	}

	if currentTopic == topicName {
		currentTopic = ""
	}

	delete(subscriptions, topicName)
	subscription.Cancel()

	delete(topics, topicName)
	if err := topic.Close(); err != nil {
		Out.Msg("local", true, "Failed to leave: %v", err)
	}
}

func connectCmd(commandParts []string) {
	if len(commandParts) != 2 {
		Out.Msg("local", true, "Usage: /connect <peer descriptor>")
		return
	}
	ma, err := multiaddr.NewMultiaddr(commandParts[1])
	if err != nil {
		Out.Msg("local", true, "Malformed address: %v", err)
		return
	}
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		Out.Msg("local", true, "Malformed address: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := node.Connect(ctx, *pi); err != nil {
		Out.Msg("local", true, "Failed to connect to %v: %v", pi.ID, err)
	} else {
		Out.Msg("local", true, "Connected to %v", pi.ID)
	}
}

func msgCmd(commandParts []string) {
	if len(commandParts) < 3 {
		Out.Msg("local", true, "Usage: /msg <descriptor> <message>")
		return
	}
	descriptor := commandParts[1]
	msg := strings.Join(commandParts[1:], " ")

	send(descriptor, msg)
}

func rejoinCmd(commandParts []string) {
	discover := discovery.NewRoutingDiscovery(kDHT)
	switch len(commandParts) {
	case 1:
		rejoinAll(discover, true)
	case 2:
		descriptor, err := expandDescriptor(commandParts[1])
		if err != nil {
			Out.Msg("local", true, "%v", err)
			return
		}
		rejoin(context.Background(), discover, true, descriptor)
	default:
		Out.Msg("local", true, "Usage: /rejoin [descriptor]")
	}
}

func peersCmd(commandParts []string) {
	Out.Msg("local", true, "Connected peers:")
	for _, p := range node.Network().Peers() {
		conns := node.Network().ConnsToPeer(p)
		for _, c := range conns {
			Out.Msg("local", true, "%v/p2p/%v", c.RemoteMultiaddr(), p)
		}
	}
}

func statCmd(commandParts []string) {
	if len(commandParts) < 2 {
		Out.Msg("local", true, "Usage: /stat <descriptor>")
		return
	}
	descriptor, err := expandDescriptor(commandParts[1])
	if err != nil {
		Out.Msg("local", true, "Invalid descriptor")
		return
	}

	switch {
	case strings.HasPrefix(descriptor, "/ip6"), strings.HasPrefix(descriptor, "/ip4"):
		statMultiaddr(descriptor)
	case strings.HasPrefix(descriptor, "/ipfs"), strings.HasPrefix(descriptor, "/p2p"):
		statMultiaddr(descriptor)
	case strings.HasPrefix(descriptor, chanPrefix):
		statChannel(descriptor)
	case strings.HasPrefix(descriptor, dmPrefix):
		statDM(descriptor)
	default:
		Out.Msg("local", true, "No idea what to do with %s", descriptor)
	}
}

func statMultiaddr(desc string) {
	ma, err := multiaddr.NewMultiaddr(desc)
	if err != nil {
		Out.Msg("local", true, "Failed to parse multiaddress: %v", err)
		return
	}
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		Out.Msg("local", true, "Failed to parse multiaddress: %v", err)
		return
	}

	statPeer(pi.ID)
}

var boolStr = map[bool]string{
	true:  "yes",
	false: "no",
}

func statPeer(peerID peer.ID) {
	Out.Msg("local", true, "Peer %v", peerID)
	info := node.Peerstore().PeerInfo(peerID)
	if len(info.Addrs) == 0 {
		Out.Msg("local", true, " Unknown peer")
		return
	}
	conns := node.Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		Out.Msg("local", true, " Not connected")
	}

	latEWMA := node.Peerstore().LatencyEWMA(peerID)
	if latEWMA != 0 {
		Out.Msg("local", true, " Latency EWMA: %v", latEWMA)
	}

	Out.Msg("local", true, "Advertised addresses:")
	for _, a := range info.Addrs {
		Out.Msg("local", true, "| %v", a)
	}
	protos, err := node.Peerstore().GetProtocols(peerID)
	if err != nil {
		Out.Msg("local", true, " GetProtocols failed: %v", err)
	} else {
		Out.Msg("local", true, "Protocols:")
		for _, prot := range protos {
			Out.Msg("local", true, "| %s", prot)
		}
	}

	if len(conns) != 0 {
		Out.Msg("local", true, "Connected via:")
		for _, c := range conns {
			Out.Msg("local", true, "| %v", c.RemoteMultiaddr())
		}
	}
}

func statChannel(desc string) {
	Out.Msg("local", true, "Channel %s", descriptorForDisplay(desc))
	Out.Msg("local", true, " Full descriptor: %s", desc)
	Out.Msg("local", true, " We are member: %s", boolStr[subscriptions[desc] != nil])
	peers := pubsubInst.ListPeers(desc)
	if len(peers) != 0 {
		Out.Msg("local", true, "Known members:")
		for _, p := range peers {
			Out.Msg("local", true, "| /p2p/%v", p)
		}
	}
}

func statDM(desc string) {
	Out.Msg("local", true, "Not implemented yet")
}

func pingCmd(commandParts []string) {
	if len(commandParts) < 2 {
		Out.Msg("local", true, "Usage: /ping <descriptor>")
		return
	}
	ma, err := multiaddr.NewMultiaddr(commandParts[1])
	if err != nil {
		Out.Msg("local", true, "Failed to parse multiaddress: %v", err)
		return
	}
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		Out.Msg("local", true, "Failed to parse multiaddress: %v", err)
		return
	}

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res := <-pingProto.Ping(ctx, pi.ID)
		Out.Msg("local", true, "RTT: %v", res.RTT)
	}
}
