package tui

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"time"

	infchat "github.com/foxcpp/infinitychat/node"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type TUI struct {
	app *tview.Application

	header *tview.TextView
	flex   *tview.Flex
	logBox *tview.TextView
	input  *tview.InputField

	logLineCount int

	inputHistory      []string
	inputHistoryIndex int

	lines chan string

	currentTopic string

	running bool
}

func New() *TUI {
	tui := &TUI{
		app:    tview.NewApplication(),
		header: tview.NewTextView(),
		flex:   tview.NewFlex(),
		logBox: tview.NewTextView(),
		input:  tview.NewInputField(),
		lines:  make(chan string, 100),
	}

	tui.header.SetBackgroundColor(tcell.Color236)
	tui.header.SetText("InfinityChat v0.1 | State: Starting...")

	tui.flex.SetDirection(tview.FlexRow)

	tui.logBox.SetBackgroundColor(tcell.Color235)
	tui.logBox.SetTextColor(tcell.Color255)
	tui.logBox.SetWrap(true)
	tui.logBox.SetDynamicColors(true)
	tui.logBox.SetWordWrap(true)
	tui.logBox.SetBorder(true)
	tui.logBox.SetBorderPadding(0, 1, 1, 1)
	io.WriteString(tui.logBox, " _        __         _           _   \n"+
		"(_)_ __  / _|    ___| |__   __ _| |_ \n"+
		"| | '_ \\| |_    / __| '_ \\ / _` | __|\n"+
		"| | | | |  _|  | (__| | | | (_| | |_ \n"+
		"|_|_| |_|_|(_)  \\___|_| |_|\\__,_|\\__|\n"+
		"InfinityChat v0.1 | Because ZeroChat is too small ;D\n\n")

	tui.flex.AddItem(tui.header, 1, 1, false)
	tui.flex.AddItem(tui.logBox, 0, 24, false)
	tui.flex.AddItem(tui.input, 1, 1, true)

	tui.input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if len(tui.inputHistory) == 0 || tui.inputHistory[len(tui.inputHistory)-1] != tui.input.GetText() {
				tui.inputHistory = append(tui.inputHistory, tui.input.GetText())
			}
			tui.inputHistoryIndex = len(tui.inputHistory)
			tui.lines <- tui.input.GetText()
			tui.input.SetText("")
		case tcell.KeyEscape:
			tui.input.SetText("")
		}
	})
	tui.input.SetFieldBackgroundColor(tcell.Color236)
	tui.input.SetFieldTextColor(tcell.Color255)
	tui.input.SetLabel("> ")
	tui.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyPgUp, tcell.KeyPgDn:
			tui.logBox.InputHandler()(event, func(tview.Primitive) {})
		case tcell.KeyUp:
			if tui.inputHistoryIndex == 0 {
				tui.input.SetText("")
				return nil
			}
			tui.inputHistoryIndex--
			tui.input.SetText(tui.inputHistory[tui.inputHistoryIndex])
		case tcell.KeyDown:
			if tui.inputHistoryIndex == len(tui.inputHistory) {
				return nil
			}
			tui.inputHistoryIndex++
			if tui.inputHistoryIndex == len(tui.inputHistory) {
				tui.input.SetText("")
				return nil
			}
			tui.input.SetText(tui.inputHistory[tui.inputHistoryIndex])
		default:
			return event
		}
		return nil
	})
	tui.input.SetLabelColor(tcell.ColorWhite)

	tui.app.SetRoot(tui.flex, true)

	return tui
}

func (tui *TUI) Run(node *infchat.Node) {
	tui.running = true
	go tui.statusUpdate(node)
	tui.app.Run()
}

func (tui *TUI) Close() error {
	tui.app.Stop()
	return nil
}

func (tui *TUI) statusUpdate(node *infchat.Node) {
	t := time.NewTicker(1 * time.Second)
	const statusLineFmt = "InfinityChat v0.1 | State: %s  %d connected peers (%d known), %d pubsub subscriptions"

	for range t.C {
		s := node.Status()

		statusLine := fmt.Sprintf(statusLineFmt, s.State, s.ConnectedPeers, s.KnownPeers, s.PubsubTopics)
		if s.NAT {
			statusLine += ", impenetrable NAT detected"
		}

		tui.app.QueueUpdateDraw(func() {
			tui.header.SetText(statusLine)
		})
	}
}

func (tui *TUI) Write(b []byte) (int, error) {
	tui.Msg("local", true, "%v", string(b))
	return 0, nil
}

func (tui *TUI) Msg(prefix string, local bool, format string, args ...interface{}) {
	tui.msg(prefix, local, true, format, args...)
}

func (tui *TUI) ColorMsg(prefix string, local bool, format string, args ...interface{}) {
	tui.msg(prefix, local, false, format, args...)
}

func (tui *TUI) Error(prefix string, local bool, format string, args ...interface{}) {
	value := fmt.Sprintf(format, args...)

	tui.msg(prefix, local, false, "[#fe3333:-:b]%s[-:-:-]", tview.Escape(value))
}

func pickColor(prefix string) string {
	if prefix == "local" {
		return `#bcbcbc`
	}

	colors := []string{
		`#60b48a`,
		`#dfaf8f`,
		`#506070`,
		`#dc8cc3`,
		`#8cd0d3`,
		`#dcdccc`,
		`#709080`,
		`#dca3a3`,
		`#c3bf9f`,
		`#f0dfaf`,
		`#94bff3`,
		`#ec93d3`,
		`#93e0e3`,
	}

	crc32 := crc32.ChecksumIEEE([]byte(prefix))
	return colors[crc32%uint32(len(colors))]
}

func (tui *TUI) msg(prefix string, local, escape bool, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	msg = strings.TrimRight(msg, "\n\t ")

	lines := strings.Split(msg, "\n")
	stamp := time.Now().Format("[#dadada]15[#8a8a8a]:[#dadada]04[#8a8a8a]:[#dadada]05[-]")

	shouldScroll := false
	scrollLine, _ := tui.logBox.GetScrollOffset()
	if scrollLine == tui.logLineCount {
		shouldScroll = true
	}

	var prefixBraces string
	if local {
		prefixBraces = tview.Escape("[" + prefix + "]")
	} else {
		prefixBraces = "<" + prefix + ">"
	}
	color := pickColor(prefix)

	var msgBuffer bytes.Buffer

	for _, line := range lines {
		if !tui.running {
			fmt.Fprintf(os.Stderr, "%v [%s] %s\n", time.Now().Format("15:04:05"), prefix, line)
		}
		fmt.Fprintf(&msgBuffer, "%v [%s][::b]%s[#eeeeee::-] %s[-]\n", stamp, color, prefixBraces, line)
		tui.logLineCount++
	}

	if shouldScroll {
		tui.logBox.ScrollToEnd()
	}

	tui.input.SetLabel(infchat.DescriptorForDisplay(tui.currentTopic) + " > ")

	tui.logBox.Write(msgBuffer.Bytes())

	if tui.running {
		tui.app.Draw()
	}
}

func (tui *TUI) ReadLine() (string, error) {
	tui.input.SetLabel(infchat.DescriptorForDisplay(tui.currentTopic) + " > ")
	return <-tui.lines, nil
}

func (tui *TUI) SetCurrentChat(desc string) {
	tui.currentTopic = desc
}

func (tui *TUI) CurrentChat() string {
	return tui.currentTopic
}
