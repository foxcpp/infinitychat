package infchat

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/foxcpp/infinitychat/errhelper"
	"github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	noise "github.com/libp2p/go-libp2p-noise"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
)

const lowConnsMark = 50
const highConnsMark = 100

type Config struct {
	Identity    ed25519.PrivateKey
	Bootstrap   []string
	ListenAddrs []string

	ConnsHigh int
	ConnsLow  int

	Log *log.Logger

	RejoinInterval time.Duration
}

type Node struct {
	Cfg Config

	Host     host.Host
	kdht     *dht.IpfsDHT
	Discover *discovery.RoutingDiscovery

	PubsubProto  *pubsub.PubSub
	PingProto    *ping.PingService
	AutonatProto autonat.AutoNAT

	// This is not perfectly fine use of context but here it is kept internally
	// and used to cancel literally everything on node shutdown.
	nodeContext context.Context
	ctxCancel   context.CancelFunc

	pubsubLock          sync.Mutex
	topics              map[string]*pubsub.Topic
	subs                map[string]*pubsub.Subscription
	knownChannelMembers map[string]int

	messages chan Message
}

func NewNode(cfg Config) (*Node, error) {
	ctx, cancel := context.WithCancel(context.Background())

	var err error
	n := &Node{
		Cfg:         cfg,
		nodeContext: ctx,
		ctxCancel:   cancel,
		messages:    make(chan Message),

		topics:              map[string]*pubsub.Topic{},
		subs:                map[string]*pubsub.Subscription{},
		knownChannelMembers: map[string]int{},
	}

	h := errhelper.New("libp2p new")
	h.Cleanup(cancel)

	// Cannot fail since it is just copying struct internally.
	privKey, _ := crypto.UnmarshalEd25519PrivateKey(cfg.Identity)

	n.Host, err = libp2p.New(
		ctx,
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(cfg.ListenAddrs...),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DefaultSecurity,
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.DefaultTransports,
		libp2p.ConnectionManager(connmgr.NewConnManager(
			lowConnsMark,
			highConnsMark,
			time.Minute, // grace
		)),
		libp2p.NATPortMap(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			n.kdht, err = dht.New(ctx, h, dht.Mode(dht.ModeAuto))
			return n.kdht, err
		}),
		libp2p.EnableAutoRelay(),
		libp2p.Ping(false), // We will configure it on our own.
		libp2p.UserAgent("infinitychat/v0.1"),
	)
	if err != nil {
		return nil, h.Fail(err)
	}
	h.CleanupClose(n.Host)

	n.Discover = discovery.NewRoutingDiscovery(n.kdht)

	n.AutonatProto, err = autonat.New(ctx, n.Host)
	if err != nil {
		return nil, h.Fail(err)
	}

	n.PubsubProto, err = pubsub.NewGossipSub(ctx, n.Host,
		pubsub.WithDiscovery(n.Discover),
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
	)
	if err != nil {
		return nil, h.Fail(err)
	}

	n.PingProto = ping.NewPingService(n.Host)

	return n, nil
}

func (n *Node) Run() {
	counter := 0
	for _, bs := range n.Cfg.Bootstrap {
		ma, err := multiaddr.NewMultiaddr(bs)
		if err != nil {
			n.Cfg.Log.Printf("Failed to parse bootstrap address: %v", err)
			return
		}
		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			n.Cfg.Log.Printf("Failed to parse bootstrap address: %v", err)
			return
		}

		ctx, cancel := context.WithTimeout(n.nodeContext, 15*time.Second)
		defer cancel()

		if err := n.Host.Connect(ctx, *pi); err != nil {
			n.Cfg.Log.Printf("Failed to connect: %v", err)
		} else {
			counter++
		}
	}

	n.Cfg.Log.Printf("Entangling fabric of infinity... %d bootstrap peers", counter)
	n.kdht.Bootstrap(n.nodeContext)
}

func (n *Node) Close() error {
	defer close(n.messages)

	n.ctxCancel()

	n.kdht.Close()

	return n.Host.Close()
}

func (n *Node) ID() peer.ID {
	return n.Host.ID()
}

func (n *Node) Ping(pid peer.ID) time.Duration {
	ctx, cancel := context.WithTimeout(n.nodeContext, 5*time.Second)
	defer cancel()

	res := <-n.PingProto.Ping(ctx, pid)
	return res.RTT
}

func (n *Node) Connect(addr multiaddr.Multiaddr) error {
	pi, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(n.nodeContext, 15*time.Second)
	defer cancel()

	if err := n.Host.Connect(ctx, *pi); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}

func (n *Node) ConnectStr(multiaddrStr string) error {
	ma, err := multiaddr.NewMultiaddr(multiaddrStr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	return n.Connect(ma)
}

func (n *Node) IsConnected(pid peer.ID) bool {
	return len(n.Host.Network().ConnsToPeer(pid)) != 0
}

type Message struct {
	Sender  peer.ID
	Channel string
	Text    string
}

func (n *Node) Messages() <-chan Message {
	return n.messages
}
