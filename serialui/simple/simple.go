package simple

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	infchat "github.com/foxcpp/infinitychat/node"
	"github.com/foxcpp/infinitychat/serialui"
)

type UI struct {
	currentBuffer string

	stopSig chan struct{}

	stdin *bufio.Scanner
}

func New() *UI {
	ui := &UI{
		stdin:   bufio.NewScanner(os.Stdin),
		stopSig: make(chan struct{}),
	}

	return ui
}

func (ui *UI) Run(node *infchat.Node) {
	<-ui.stopSig
}

func (ui *UI) Close() error {
	os.Stdin.Close()
	ui.stopSig <- struct{}{}
	return nil
}

func (ui *UI) Write(b []byte) (int, error) {
	ui.Msg("", "local", "%v", string(b))
	return 0, nil
}

func (ui *UI) Msg(buffer, sender string, format string, args ...interface{}) {
	ui.msg(buffer, sender, true, format, args...)
}

func (ui *UI) ColorMsg(buffer, sender string, format string, args ...interface{}) {
	ui.msg(buffer, sender, false, format, args...)
}

func (ui *UI) Error(buffer, format string, args ...interface{}) {
	value := fmt.Sprintf(format, args...)

	ui.msg(buffer, "local", false, "%s", value)
}

func (ui *UI) msg(buffer, sender string, escape bool, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	msg = strings.TrimRight(msg, "\n\t ")

	lines := strings.Split(msg, "\n")
	stamp := time.Now().Format("15:04:05")

	var prefixBraces string
	if sender == "local" {
		prefixBraces = "[local]"
	} else if buffer == "" || buffer == ui.CurrentBuffer() {
		prefixBraces = "<" + sender + ">"
	} else {
		prefixBraces = "<" + buffer + ":" + sender + ">"
	}

	var msgBuffer bytes.Buffer
	for _, line := range lines {
		fmt.Fprintf(&msgBuffer, "%v %s %s\n", stamp, prefixBraces, line)
	}

	os.Stderr.Write(msgBuffer.Bytes())
}

func (ui *UI) ReadLine() (string, string, error) {
	if !ui.stdin.Scan() {
		if err := ui.stdin.Err(); err != nil {
			if err == os.ErrClosed {
				return "", "", serialui.ErrInterrupt
			}
			return "", "", err
		}
		return "", "", serialui.ErrInterrupt
	}

	return ui.currentBuffer, ui.stdin.Text(), nil
}

func (ui *UI) SetCurrentBuffer(desc string) {
	ui.currentBuffer = desc
}

func (ui *UI) CurrentBuffer() string {
	return ui.currentBuffer
}
