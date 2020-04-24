package ircd

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	infchat "github.com/foxcpp/infinitychat/node"
	"github.com/foxcpp/infinitychat/serialui"
	"gopkg.in/irc.v3"
)

type conn struct {
	*irc.Conn
	Net net.Conn
}

type UI struct {
	stopSig chan struct{}
	lines   chan struct{ buf, line string }

	l net.Listener

	connsLck sync.Mutex
	conns    map[string]conn
	joined   map[string]map[string]conn

	Log  *log.Logger
	Node *infchat.Node
}

func New(listen string, logger *log.Logger) *UI {
	ui := &UI{
		lines:   make(chan struct{ buf, line string }, 100),
		stopSig: make(chan struct{}),
		Log:     logger,
		conns:   make(map[string]conn),
		joined:  make(map[string]map[string]conn),
	}

	l, err := net.Listen("tcp", listen)
	if err != nil {
		logger.Println("Cannot listen:", err)
		return nil
	}

	ui.l = l

	return ui
}

func (ui *UI) Run(node *infchat.Node) {
	ui.Node = node
	for {
		conn, err := ui.l.Accept()
		if err != nil {
			break
		}

		go ui.handleConn(conn)
	}

	for _, c := range ui.conns {
		c.Net.Close()
	}
}

func (ui *UI) Close() error {
	ui.l.Close()
	close(ui.stopSig)
	return nil
}

func (ui *UI) Write(b []byte) (int, error) {
	ui.Msg("", "local", "%v", string(b))
	return 0, nil
}

func (ui *UI) Msg(buffer, sender string, format string, args ...interface{}) {
	ui.msg(buffer, sender, format, args...)
}

func (ui *UI) ColorMsg(buffer, sender string, format string, args ...interface{}) {
	ui.msg(buffer, sender, format, args...)
}

func (ui *UI) Error(buffer, format string, args ...interface{}) {
	value := fmt.Sprintf(format, args...)

	ui.msg(buffer, "local", "%s", value)
}

func (ui *UI) handleConn(netConn net.Conn) {
	c := conn{
		Conn: irc.NewConn(netConn),
		Net:  netConn,
	}
	defer c.Net.Close()
	connID := c.Net.RemoteAddr().String()

	servPrefix := &irc.Prefix{
		Name: "infinitychat.invalid",
	}
	clPrefix := &irc.Prefix{
		Name: ui.Node.ID().String(),
	}

	errors := 0
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			if errors == 3 {
				ui.connsLck.Lock()
				delete(ui.conns, connID)
				for _, buf := range ui.joined {
					delete(buf, connID)
				}
				ui.connsLck.Unlock()
				return
			}
			errors++
		}
		if msg == nil {
			// wtf
			return
		}

		switch msg.Command {
		case "NICK":
			// No-op, we don't care about nickname client uses.
		case "USER":
			// Complete IRC "registration" dance.
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "001",
				Params:  []string{clPrefix.Name, "Welcome, idiot"},
			})
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "002",
				Params:  []string{"InfinityChat node version 0.1"},
			})
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "004",
				Params:  []string{"infinitychat.invalid", "infchat", "v0.1", "", ""},
			})
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "005",
				Params:  []string{"CHANTYPES=#", "NETWORK=infchat", "CASEMAPPING=rfc1459" /* lie */, "CHARSET=ascii", "NICKLEN=256", "CHANNELLEN=512", "TOPICLEN=1" /* also lie */},
			})
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "251",
				Params:  []string{strconv.Itoa(len(ui.Node.Host.Network().Conns())), "connected peers"},
			})
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "252",
				Params:  []string{strconv.Itoa(len(ui.conns)), "clients connected to IRC gateway"},
			})
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "254",
				Params: []string{
					strconv.Itoa(len(ui.Node.PubsubProto.GetTopics())),
					"pubsub subscriptions",
				},
			})
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "422",
				Params: []string{
					"no MOTD for you",
				},
			})
			ui.connsLck.Lock()
			ui.conns[connID] = c
			ui.connsLck.Unlock()
		case "PRIVMSG":
			if msg.Params[0] == "local" {
				ui.lines <- struct{ buf, line string }{
					buf:  "irc_conn:" + connID,
					line: "/" + msg.Params[1],
				}
			} else {
				ui.lines <- struct{ buf, line string }{
					buf:  "irc_conn:" + connID,
					line: "/msg " + msg.Params[0] + " " + msg.Params[1],
				}
			}
		case "JOIN":
			ui.lines <- struct{ buf, line string }{
				buf:  "irc_conn:" + connID,
				line: "/join " + msg.Params[0],
			}
			ui.connsLck.Lock()
			if ui.joined[msg.Params[0]] == nil {
				ui.joined[msg.Params[0]] = make(map[string]conn)
			}
			ui.joined[msg.Params[0]][connID] = c
			ui.connsLck.Unlock()
			c.WriteMessage(&irc.Message{
				Prefix:  clPrefix,
				Command: "JOIN",
				Params:  []string{msg.Params[0]},
			})
			fallthrough
		case "NAMES":
			descr, err := infchat.ExpandDescriptor(msg.Params[0])
			if err != nil {
				// what do I do...
			}
			members := []string{ui.Node.ID().String()}
			for _, peer := range ui.Node.ConnectedMembers(descr) {
				members = append(members, peer.String())
			}
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "353",
				Params:  []string{clPrefix.Name, "=", msg.Params[0], strings.Join(members, " ")},
			})
		case "PART":
			ui.lines <- struct{ buf, line string }{
				buf:  "irc_conn:" + connID,
				line: "/leave " + msg.Params[0],
			}
			ui.connsLck.Lock()
			delete(ui.joined, msg.Params[0])
			ui.connsLck.Unlock()
			c.WriteMessage(&irc.Message{
				Prefix:  clPrefix,
				Command: "PART",
				Params:  []string{msg.Params[0]},
			})
		case "PING":
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "PONG",
				Params:  []string{servPrefix.Name, servPrefix.Name},
			})
		case "QUIT":
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "ERROR",
				Params: []string{
					"so we are sending ERROR on correct connection closure, ok, IRCv3",
				},
			})
			ui.connsLck.Lock()
			delete(ui.conns, connID)
			for _, buf := range ui.joined {
				delete(buf, connID)
			}
			ui.connsLck.Unlock()
		default:
			ui.Log.Printf("Not implemented command: %s %s", msg.Command, msg.Params)
			c.WriteMessage(&irc.Message{
				Prefix:  servPrefix,
				Command: "421",
				Params:  []string{"Not implemented"},
			})
		}
	}
}

func (ui *UI) msg(buffer, sender string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	lines := strings.Split(msg, "\n")
	for _, line := range lines {
		ui.msgLine(buffer, sender, line)
	}
}

func (ui *UI) msgLine(buffer, sender, line string) {
	if buffer == "" {
		return
	}
	if sender == ui.Node.ID().String() {
		return
	}

	ui.connsLck.Lock()
	defer ui.connsLck.Unlock()

	ui.Log.Printf("%s <<< %s: %s", buffer, sender, line)

	if strings.HasPrefix(buffer, "irc_conn:") {
		connID := strings.TrimPrefix(buffer, "irc_conn:")
		conn := ui.conns[connID]
		if conn.Net == nil {
			// wtf...
			return
		}

		conn.Net.SetWriteDeadline(time.Now().Add(5 * time.Second))
		conn.WriteMessage(&irc.Message{
			Prefix: &irc.Prefix{
				Name: sender,
			},
			Command: "NOTICE",
			Params:  []string{buffer, line},
		})
		conn.Net.SetWriteDeadline(time.Time{})
		return
	}

	if buffer == "" {
		for _, conn := range ui.joined[buffer] {
			conn.Net.SetWriteDeadline(time.Now().Add(5 * time.Second))
			conn.WriteMessage(&irc.Message{
				Prefix: &irc.Prefix{
					Name: sender,
				},
				Command: "PRIVMSG",
				Params:  []string{line},
			})
			conn.Net.SetWriteDeadline(time.Time{})
		}
	}

	for connID, c := range ui.joined[buffer] {
		c.Net.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := c.WriteMessage(&irc.Message{
			Prefix: &irc.Prefix{
				Name: sender,
			},
			Command: "PRIVMSG",
			Params:  []string{buffer, line},
		})
		if err != nil {
			c.Net.Close()
			delete(ui.joined[buffer], connID)
			delete(ui.conns, connID)
			ui.Log.Printf("IRC: I/O error, dropped connection %s: %v", connID, err)
		}
		c.Net.SetWriteDeadline(time.Time{})
	}
}

func (ui *UI) ReadLine() (string, string, error) {
	line, ok := <-ui.lines
	if !ok {
		return "", "", serialui.ErrInterrupt
	}
	ui.Log.Printf("%s >>> %s", line.buf, line.line)
	return line.buf, line.line, nil
}

func (ui *UI) SetCurrentBuffer(desc string) {
	// no-op
}

func (ui *UI) CurrentBuffer() string {
	// no-op
	return ""
}
