package serialui

import (
	"strings"

	infchat "github.com/foxcpp/infinitychat/node"
)

func InputLoop(ui UI, node *infchat.Node) {
	for {
		l, err := ui.ReadLine()
		if err != nil {
			if err == ErrInterrupt {
				return
			}
			ui.Error("local", true, "I/O error: %v", err)
			return
		}

		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "/") {
			if ui.CurrentChat() == "" {
				ui.Msg("local", true, "You shout in the empty field with noone to hear you... use /join <channel>")
				continue
			}
			node.Post(ui.CurrentChat(), t)

			ui.Msg(node.ID().String(), true, t)
			continue
		}

		if err := HandleCommand(ui, node, t); err != nil {
			if err == ErrInterrupt {
				ui.Close()
				return
			}
			ui.Error("local", true, "%v", err)
		}
	}
}

func PullMessages(ui UI, node *infchat.Node) {
	for msg := range node.Messages() {
		if ui.CurrentChat() == msg.Channel {
			ui.Msg(msg.Sender.String(), false, "%s", msg.Text)
		} else {
			ui.Msg(infchat.DescriptorForDisplay(msg.Channel)+":"+msg.Sender.String(), false, "%s", msg.Text)
		}
	}
}
