package serialui

import (
	"errors"
	"sort"
	"strings"

	infchat "github.com/foxcpp/infinitychat/node"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

var ErrInterrupt = errors.New("interrupt requested")

func HandleCommand(ui UI, node *infchat.Node, line string) error {
	type cmd struct {
		Description string
		FullHelp    string
		Callback    func(UI, *infchat.Node, []string)
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
			Description: "Force DHT lookup of channel members",
			FullHelp: `/rejoin [channel descriptor]

Might help to accelerate mesh reconnection in case of nodes falling offline.`,
			Callback: rejoinCmd,
		},
		"announce": {
			Description: "Force announce of channel membership",
			FullHelp: `/rejoin [channel descriptor]

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
			Callback: func(_ UI, _ *infchat.Node, p []string) {
				if len(p) != 1 {
					ui.Msg("local", true, "Usage: /id")
				}
				ui.Msg("local", true, "my ID: %v", node.ID())
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
		"listen": {
			Description: "Show current listening addresses",
			Callback:    listenCmd,
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
		Callback: func(_ UI, _ *infchat.Node, parts []string) {
			switch len(parts) {
			case 1:
				ui.Msg("local", true, "Available commands:")
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
					ui.Msg("local", true, "/%s%s%s", cmd, strings.Repeat(" ", maxLen-len(cmd)+4), cmds[cmd].Description)
				}
			case 2:
				key := strings.ToLower(strings.TrimPrefix(parts[1], "/"))
				info, ok := cmds[key]
				if !ok {
					ui.Error("local", true, "Unknown command")
					return
				}
				ui.Msg("local", true, "%s\n%s", info.Description, info.FullHelp)
			default:
				ui.Msg("local", true, "usage: /help [command]")
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
		ui.Msg("local", true, "Unknown command /%s, try /help", key)
		return nil
	}
	cb.Callback(ui, node, parts)

	return nil
}

func joinCmd(ui UI, node *infchat.Node, commandParts []string) {
	if len(commandParts) != 2 {
		ui.Msg("local", true, "Usage: /join <channel descriptor>")
		return
	}
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Error("local", true, "Invalid channel descriptor")
		return
	}

	if err := node.JoinChannel(descriptor); err != nil {
		ui.Error("local", true, "Join failed: %v", err)
		return
	}

	ui.Msg("local", true, "Joined %s", commandParts[1])
	ui.SetCurrentChat(descriptor)
}

func leaveCmd(ui UI, node *infchat.Node, commandParts []string) {
	if len(commandParts) != 2 {
		ui.Msg("local", true, "Usage: /leave <channel descriptor>")
		return
	}
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Error("local", true, "Invalid channel descriptor")
		return
	}

	if err := node.LeaveChannel(descriptor); err != nil {
		ui.Error("local", true, "Leave failed: %v", err)
		return
	}

	if ui.CurrentChat() == descriptor {
		ui.SetCurrentChat("")
	}

	ui.Msg("local", true, "Left %s", commandParts[1])
}

func connectCmd(ui UI, node *infchat.Node, commandParts []string) {
	if len(commandParts) != 2 {
		ui.Msg("local", true, "Usage: /connect <peer descriptor>")
		return
	}

	pid, err := node.ConnectStr(commandParts[1])
	if err != nil {
		ui.Error("local", true, "Connect failed: %v", err)
		return
	}
	ui.Msg("local", true, "Connected to %s", pid)
}

func listenCmd(ui UI, node *infchat.Node, commandParts []string) {
	ui.Msg("local", true, "Listening on:")
	for _, ma := range node.Host.Addrs() {
		ui.Msg("local", true, "%v", ma)
	}
}

func msgCmd(ui UI, node *infchat.Node, commandParts []string) {
	if len(commandParts) < 3 {
		ui.Msg("local", true, "Usage: /msg <descriptor> <message>")
		return
	}
	descriptor := commandParts[1]
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Error("local", true, "Invalid channel descriptor")
		return
	}
	msg := strings.Join(commandParts[1:], " ")

	if err := node.Post(descriptor, msg); err != nil {
		ui.Error("local", true, "Post failed: %v", err)
		return
	}

	if ui.CurrentChat() == descriptor {
		ui.Msg(node.ID().String(), true, msg)
	} else {
		ui.Msg(infchat.DescriptorForDisplay(descriptor)+":"+node.ID().String(), true, msg)
	}
}

func rejoinCmd(ui UI, node *infchat.Node, commandParts []string) {
	go func() {
		var err error
		switch len(commandParts) {
		case 1:
			err = node.RejoinAll()
		case 2:
			descriptor, err := infchat.ExpandDescriptor(commandParts[1])
			if err != nil {
				ui.Error("local", true, "%v", err)
				return
			}
			err = node.RejoinChannel(descriptor)
		default:
			ui.Msg("local", true, "Usage: /rejoin [descriptor]")
			return
		}
		if err != nil {
			ui.Error("local", true, "%v", err)
		}
	}()
}

func announceCmd(ui UI, node *infchat.Node, commandParts []string) {
	go func() {
		var err error
		switch len(commandParts) {
		case 1:
			err = node.AnnounceAll()
		case 2:
			descriptor, err := infchat.ExpandDescriptor(commandParts[1])
			if err != nil {
				ui.Error("local", true, "%v", err)
				return
			}
			err = node.AnnounceChannel(descriptor)
		default:
			ui.Msg("local", true, "Usage: /announce [descriptor]")
			return
		}
		if err != nil {
			ui.Error("local", true, "%v", err)
		}
	}()
}

func peersCmd(ui UI, node *infchat.Node, commandParts []string) {
	ui.Msg("local", true, "Connected peers:")
	for _, p := range node.Host.Network().Peers() {
		conns := node.Host.Network().ConnsToPeer(p)
		for _, c := range conns {
			ui.Msg("local", true, "%v/p2p/%v", c.RemoteMultiaddr(), p)
		}
	}
}

func statCmd(ui UI, node *infchat.Node, commandParts []string) {
	if len(commandParts) < 2 {
		ui.Msg("local", true, "Usage: /stat <descriptor>")
		return
	}
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Msg("local", true, "Invalid descriptor")
		return
	}

	switch {
	case strings.HasPrefix(descriptor, "/ip6"), strings.HasPrefix(descriptor, "/ip4"):
		statMultiaddr(ui, node, descriptor)
	case strings.HasPrefix(descriptor, "/ipfs"), strings.HasPrefix(descriptor, "/p2p"):
		statMultiaddr(ui, node, descriptor)
	case strings.HasPrefix(descriptor, infchat.ChanPrefix):
		statChannel(ui, node, descriptor)
	case strings.HasPrefix(descriptor, infchat.DMPrefix):
		statDM(ui, node, descriptor)
	default:
		ui.Msg("local", true, "No idea what to do with %s", descriptor)
	}
}

func statMultiaddr(ui UI, node *infchat.Node, desc string) {
	ma, err := multiaddr.NewMultiaddr(desc)
	if err != nil {
		ui.Msg("local", true, "Failed to parse multiaddress: %v", err)
		return
	}
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		ui.Msg("local", true, "Failed to parse multiaddress: %v", err)
		return
	}

	statPeer(ui, node, pi.ID)
}

var boolStr = map[bool]string{
	true:  "yes",
	false: "no",
}

func statPeer(ui UI, node *infchat.Node, peerID peer.ID) {
	ui.Msg("local", true, "Peer /p2p/%v", peerID)
	info := node.Host.Peerstore().PeerInfo(peerID)
	if len(info.Addrs) == 0 {
		ui.Msg("local", true, " Unknown peer")
		return
	}
	conns := node.Host.Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		ui.Msg("local", true, " Not connected")
	}

	latEWMA := node.Host.Peerstore().LatencyEWMA(peerID)
	if latEWMA != 0 {
		ui.Msg("local", true, " Latency EWMA: %v", latEWMA)
	}

	ui.Msg("local", true, "Advertised addresses:")
	for _, a := range info.Addrs {
		ui.Msg("local", true, "| %v", a)
	}
	protos, err := node.Host.Peerstore().GetProtocols(peerID)
	if err != nil {
		ui.Msg("local", true, " GetProtocols failed: %v", err)
	} else {
		ui.Msg("local", true, "Protocols:")
		for _, prot := range protos {
			ui.Msg("local", true, "| %s", prot)
		}
	}

	if len(conns) != 0 {
		ui.Msg("local", true, "Connected via:")
		for _, c := range conns {
			ui.Msg("local", true, "| %v", c.RemoteMultiaddr())
		}
	}
}

func statChannel(ui UI, node *infchat.Node, desc string) {
	ui.Msg("local", true, "Channel %s", infchat.DescriptorForDisplay(desc))
	ui.Msg("local", true, " Full descriptor: %s", desc)
	ui.Msg("local", true, " We are member: %s", boolStr[node.IsJoined(desc)])
	peers := node.ConnectedMembers(desc)
	if len(peers) != 0 {
		ui.Msg("local", true, "Connected members:")
		for _, p := range peers {
			node.Host.Network().Connectedness(p)
			ui.Msg("local", true, "| /p2p/%v", p)
		}
	}
}

func statDM(ui UI, node *infchat.Node, desc string) {
	ui.Msg("local", true, "Not implemented yet")
}

func pingCmd(ui UI, node *infchat.Node, commandParts []string) {
	if len(commandParts) < 2 {
		ui.Msg("local", true, "Usage: /ping <peer ID>")
		return
	}

	pid, err := peer.Decode(commandParts[1])
	if err != nil {
		ui.Error("local", true, "Malformed ID: %v", err)
		return
	}

	for i := 0; i < 3; i++ {
		res := node.Ping(pid)
		ui.Msg("local", true, "RTT to %v: %v", pid, res)
	}
}
