package serialui

import (
	"strings"

	infchat "github.com/foxcpp/infinitychat/node"
)

func InputLoop(ui UI, node *infchat.Node) {
	for {
		bufferName, l, err := ui.ReadLine()
		if err != nil {
			if err == ErrInterrupt {
				return
			}
			ui.Error(bufferName, "I/O error: %v", err)
			return
		}

		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "/") {
			if bufferName == "" {
				ui.Msg(bufferName, "local", "You shout in the empty field with noone to hear you... use /join <channel>")
				continue
			}
			descr, err := infchat.ExpandDescriptor(bufferName)
			if err != nil {
				ui.Error(bufferName, "Post failed: invalid buffer: %v", err)
				continue
			}
			if err := node.Post(descr, t); err != nil {
				ui.Error(bufferName, "Post failed: %v", err)
				continue
			}
			ui.Msg(bufferName, node.ID().String(), t)
			continue
		}

		if err := HandleCommand(ui, node, bufferName, t); err != nil {
			if err == ErrInterrupt {
				ui.Close()
				return
			}
			ui.Error(bufferName, "%v", err)
		}
	}
}

func PullMessages(ui UI, node *infchat.Node) {
	for msg := range node.Messages() {
		ui.Msg(infchat.DescriptorForDisplay(msg.Channel), msg.Sender.String(), "%s", msg.Text)
	}
}
