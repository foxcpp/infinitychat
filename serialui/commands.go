package serialui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	infchat "github.com/foxcpp/infinitychat/node"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

var ErrInterrupt = errors.New("interrupt requested")

func HandleCommand(ui UI, node *infchat.Node, buffer, line string) error {
	type cmd struct {
		Description string
		FullHelp    string
		Callback    func(UI, *infchat.Node, string, []string)
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
			Callback: func(_ UI, _ *infchat.Node, buf string, p []string) {
				if len(p) != 1 {
					ui.Msg(buf, "local", "Usage: /id")
				}
				ui.Msg(buf, "local", "My ID: %v", node.ID())
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
		Callback: func(_ UI, _ *infchat.Node, buf string, parts []string) {
			switch len(parts) {
			case 1:
				ui.Msg(buf, "local", "Available commands:")
				cmdList := make([]string, 0, len(cmds))
				maxLen := 0
				for cmd := range cmds {
					if len(cmd) > maxLen {
						maxLen = len(cmd)
					}
					cmdList = append(cmdList, cmd)
				}
				var msgBuffer strings.Builder
				// Put into slice and sort to give it determistic ordering.
				sort.Strings(cmdList)
				for _, cmd := range cmdList {
					fmt.Fprintf(&msgBuffer, "/%s%s%s\n", cmd, strings.Repeat(" ", maxLen-len(cmd)+4), cmds[cmd].Description)
				}
				ui.Msg(buf, "local", msgBuffer.String())
			case 2:
				key := strings.ToLower(strings.TrimPrefix(parts[1], "/"))
				info, ok := cmds[key]
				if !ok {
					ui.Error(buf, "local", "Unknown command")
					return
				}
				ui.Msg(buf, "local", "%s\n%s", info.Description, info.FullHelp)
			default:
				ui.Msg(buf, "local", "Usage: /help [command]")
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
		ui.Msg(buffer, "local", "Unknown command /%s, try /help", key)
		return nil
	}
	cb.Callback(ui, node, buffer, parts)

	return nil
}

func joinCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	if len(commandParts) != 2 {
		ui.Msg(buf, "local", "Usage: /join <channel descriptor>")
		return
	}
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Error(buf, "local", "Invalid channel descriptor")
		return
	}

	if err := node.JoinChannel(descriptor); err != nil {
		ui.Error(buf, "local", "Join failed: %v", err)
		return
	}

	ui.Msg(buf, "local", "Joined %s", commandParts[1])
	ui.SetCurrentBuffer(infchat.DescriptorForDisplay(descriptor))
}

func leaveCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	if len(commandParts) != 2 {
		ui.Msg(buf, "local", "Usage: /leave <channel descriptor>")
		return
	}
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Error(buf, "local", "Invalid channel descriptor")
		return
	}

	if err := node.LeaveChannel(descriptor); err != nil {
		ui.Error(buf, "local", "Leave failed: %v", err)
		return
	}

	if ui.CurrentBuffer() == infchat.DescriptorForDisplay(descriptor) {
		ui.SetCurrentBuffer("")
	}

	ui.Msg(buf, "local", "Left %s", commandParts[1])
}

func connectCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	if len(commandParts) != 2 {
		ui.Msg(buf, "local", "Usage: /connect <peer descriptor>")
		return
	}

	pid, err := node.ConnectStr(commandParts[1])
	if err != nil {
		ui.Error(buf, "local", "Connect failed: %v", err)
		return
	}
	ui.Msg(buf, "local", "Connected to %s", pid)
}

func listenCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	var buffer strings.Builder

	buffer.WriteString("Listening on:\n")
	for _, ma := range node.Host.Addrs() {
		buffer.WriteString(ma.String())
		buffer.WriteRune('\n')
	}

	ui.Msg(buf, "local", buffer.String())
}

func msgCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	if len(commandParts) < 3 {
		ui.Msg(buf, "local", "Usage: /msg <descriptor> <message>")
		return
	}
	descriptor := commandParts[1]
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Error(buf, "local", "Invalid channel descriptor")
		return
	}
	msg := strings.Join(commandParts[2:], " ")

	if err := node.Post(descriptor, msg); err != nil {
		ui.Error(buf, "local", "Post failed: %v", err)
		return
	}

	ui.Msg(infchat.DescriptorForDisplay(descriptor), node.ID().String(), msg)
}

func rejoinCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	go func() {
		var err error
		switch len(commandParts) {
		case 1:
			err = node.RejoinAll()
		case 2:
			descriptor, err := infchat.ExpandDescriptor(commandParts[1])
			if err != nil {
				ui.Error(buf, "local", "%v", err)
				return
			}
			err = node.RejoinChannel(descriptor)
		default:
			ui.Msg(buf, "local", "Usage: /rejoin [descriptor]")
			return
		}
		if err != nil {
			ui.Error(buf, "%v", err)
		}
	}()
}

func announceCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	go func() {
		var err error
		switch len(commandParts) {
		case 1:
			err = node.AnnounceAll()
		case 2:
			descriptor, err := infchat.ExpandDescriptor(commandParts[1])
			if err != nil {
				ui.Error(buf, "%v", err)
				return
			}
			err = node.AnnounceChannel(descriptor)
		default:
			ui.Msg(buf, "local", "Usage: /announce [descriptor]")
			return
		}
		if err != nil {
			ui.Error(buf, "%v", err)
		}
	}()
}

func peersCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	var msg strings.Builder

	fmt.Fprintf(&msg, "Connected peers:\n")
	for _, p := range node.Host.Network().Peers() {
		conns := node.Host.Network().ConnsToPeer(p)
		for _, c := range conns {
			fmt.Fprintf(&msg, "%v/p2p/%v\n", c.RemoteMultiaddr(), p)
		}
	}

	ui.Msg(buf, "local", "%s", msg.String())
}

func statCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	if len(commandParts) < 2 {
		ui.Msg(buf, "local", "Usage: /stat <descriptor>")
		return
	}
	descriptor, err := infchat.ExpandDescriptor(commandParts[1])
	if err != nil {
		ui.Error(buf, "local", "Invalid descriptor")
		return
	}

	switch {
	case strings.HasPrefix(descriptor, "/ip6"), strings.HasPrefix(descriptor, "/ip4"):
		statMultiaddr(ui, node, buf, descriptor)
	case strings.HasPrefix(descriptor, "/ipfs"), strings.HasPrefix(descriptor, "/p2p"):
		statMultiaddr(ui, node, buf, descriptor)
	case strings.HasPrefix(descriptor, infchat.ChanPrefix):
		statChannel(ui, node, buf, descriptor)
	case strings.HasPrefix(descriptor, infchat.DMPrefix):
		statDM(ui, node, buf, descriptor)
	default:
		ui.Msg(buf, "local", "No idea what to do with %s", descriptor)
	}
}

func statMultiaddr(ui UI, node *infchat.Node, buf, desc string) {
	ma, err := multiaddr.NewMultiaddr(desc)
	if err != nil {
		ui.Error(buf, "local", "Failed to parse multiaddress: %v", err)
		return
	}
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		ui.Error(buf, "local", "Failed to parse multiaddress: %v", err)
		return
	}

	statPeer(ui, node, buf, pi.ID)
}

var boolStr = map[bool]string{
	true:  "yes",
	false: "no",
}

func statPeer(ui UI, node *infchat.Node, buf string, peerID peer.ID) {
	var msg strings.Builder

	fmt.Fprintf(&msg, "Peer /p2p/%v\n", peerID)
	info := node.Host.Peerstore().PeerInfo(peerID)
	if len(info.Addrs) == 0 {
		fmt.Fprintf(&msg, " Unknown peer\n")
		ui.Msg(buf, "local", msg.String())
		return
	}
	conns := node.Host.Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		fmt.Fprintf(&msg, " Not connected\n")
	}

	latEWMA := node.Host.Peerstore().LatencyEWMA(peerID)
	if latEWMA != 0 {
		fmt.Fprintf(&msg, " Latency EWMA: %v\n", latEWMA)
	}

	fmt.Fprintf(&msg, "Advertised addresses:\n")
	for _, a := range info.Addrs {
		fmt.Fprintf(&msg, "| %v\n", a)
	}
	protos, err := node.Host.Peerstore().GetProtocols(peerID)
	if err != nil {
		fmt.Fprintf(&msg, " GetProtocols failed: %v\n", err)
	} else {
		fmt.Fprintf(&msg, "Protocols:\n")
		for _, prot := range protos {
			fmt.Fprintf(&msg, "| %s\n", prot)
		}
	}

	if len(conns) != 0 {
		fmt.Fprintf(&msg, "Connected via:\n")
		for _, c := range conns {
			fmt.Fprintf(&msg, "| %v\n", c.RemoteMultiaddr())
		}
	}

	ui.Msg(buf, "local", "%s", msg.String())
}

func statChannel(ui UI, node *infchat.Node, buf string, desc string) {
	var msg strings.Builder

	fmt.Fprintf(&msg, "Channel %s\n", infchat.DescriptorForDisplay(desc))
	fmt.Fprintf(&msg, " Full descriptor: %s\n", desc)
	fmt.Fprintf(&msg, " We are member: %s\n", boolStr[node.IsJoined(desc)])
	peers := node.ConnectedMembers(desc)
	if len(peers) != 0 {
		fmt.Fprintf(&msg, "Connected members:\n")
		for _, p := range peers {
			fmt.Fprintf(&msg, "| /p2p/%v", p)
		}
	}

	ui.Msg(buf, "local", "%s", msg.String())
}

func statDM(ui UI, node *infchat.Node, buf, desc string) {
	ui.Msg(buf, "local", "Not implemented yet")
}

func pingCmd(ui UI, node *infchat.Node, buf string, commandParts []string) {
	if len(commandParts) < 2 {
		ui.Msg(buf, "local", "Usage: /ping <peer ID>")
		return
	}

	pid, err := peer.Decode(commandParts[1])
	if err != nil {
		ui.Error(buf, "Malformed ID: %v", err)
		return
	}

	for i := 0; i < 3; i++ {
		res := node.Ping(pid)
		ui.Msg(buf, "local", "RTT to %v: %v", pid, res)
	}
}
