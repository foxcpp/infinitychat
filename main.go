package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-kad-dht"
	noise "github.com/libp2p/go-libp2p-noise"
	"github.com/libp2p/go-libp2p-pubsub"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	routing "github.com/libp2p/go-libp2p-routing"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/crypto/ssh/terminal"
)

func loadKey(path string) (crypto.PrivKey, error) {
	seed := make([]byte, ed25519.SeedSize)
	privKeyBase64, err := ioutil.ReadFile(path)
	if err == nil {
		decodedLen, err := base64.StdEncoding.Decode(seed, privKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("loadKey: %w", err)
		}
		if decodedLen != ed25519.SeedSize {
			return nil, fmt.Errorf("loadKey: invalid private key length")
		}

		privKey := ed25519.NewKeyFromSeed(seed)
		privKeylibp2p, err := crypto.UnmarshalEd25519PrivateKey(privKey)
		if err != nil {
			return nil, fmt.Errorf("loadKey: %w", err)
		}
		return privKeylibp2p, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("loadKey: %w", err)
	}

	Out.Msg("local", true, "Generating a new ed25519 key pair...")

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("loadKey: %w", err)
	}
	if err := ioutil.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(privKey.Seed())), 0600); err != nil {
		return nil, fmt.Errorf("loadKey: %w", err)
	}

	privKeylibp2p, err := crypto.UnmarshalEd25519PrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("loadKey: %w", err)
	}
	return privKeylibp2p, nil
}

var (
	node       host.Host
	kDHT       *dht.IpfsDHT
	autonatSys autonat.AutoNAT
	pingProto  *ping.PingService

	pubsubInst    *pubsub.PubSub
	topics        = make(map[string]*pubsub.Topic)
	topicsLock    sync.Mutex
	subscriptions = make(map[string]*pubsub.Subscription)

	currentTopic string
)

var bootstrap = []string{
	//"/ip4/51.15.110.221/tcp/4001/ipfs/QmZBXSZw6qwBhqiiv6xSJJQyrC6neyz3BTjGdyTa9sovKt",
	//"/ip6/2001:bc8:1840:724::1/tcp/4001/ipfs/QmZBXSZw6qwBhqiiv6xSJJQyrC6neyz3BTjGdyTa9sovKt",
}

func canLog() bool {
	if os.Getenv("GOLOG_FILE") != "" {
		return true
	}

	return !terminal.IsTerminal(int(os.Stderr.Fd()))
}

const (
	lowConnsMark  = 30
	highConnsMark = 50
)

func main() {
	port := flag.String("port", "23120", "Local port to use for libp2p communication")
	keyFile := flag.String("key", "infinitychat.key", "Private key file to use")
	flag.Parse()

	tui := NewTUI()
	Out = tui
	go tui.Run()

	if canLog() {
		log.SetAllLoggers(log.LevelInfo)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	key, err := loadKey(*keyFile)
	if err != nil {
		Out.Msg("local", true, "%v", err)
		return
	}

	node, err = libp2p.New(
		ctx,
		libp2p.Identity(key),
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/"+*port,
			"/ip6/::/tcp/"+*port,
			"/ip4/0.0.0.0/udp/"+*port+"/quic",
			"/ip6/::/udp/"+*port+"/quic",
		),
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
			kDHT, err = dht.New(ctx, h)
			return kDHT, err
		}),
		libp2p.EnableAutoRelay(),
		libp2p.Ping(false),
		libp2p.UserAgent("infinitychat/v0.1"),
	)
	if err != nil {
		Out.Msg("local", true, "%v", err)
		return
	}
	defer node.Close()

	autonatSys, err = autonat.New(ctx, node)
	if err != nil {
		Out.Msg("local", true, "%v", err)
		return
	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			if s := autonatSys.Status(); s != network.ReachabilityUnknown {
				if s == network.ReachabilityPublic {
					pubAddr, err := autonatSys.PublicAddr()
					if err != nil {
						Out.Msg("local", true, "Failed to get public IPs: %v", err)
						return
					}
					Out.Msg("local", true, "Discovered node public addresses: %v", pubAddr)
				}
				return
			}
		}
	}()

	Out.Msg("local", true, "My ID: %s", node.ID())

	prepareBootstrap()

	pubsubInst, err = pubsub.NewGossipSub(ctx, node,
		pubsub.WithDiscovery(discovery.NewRoutingDiscovery(kDHT)),
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
	)
	if err != nil {
		Out.Msg("local", true, "%v", err)
		return
	}

	pingProto = ping.NewPingService(node)

	quit := make(chan struct{})

	go inputLoop(quit)
	go announceTick()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	select {
	case <-sig:
	case <-quit:
	}
}

func prepareBootstrap() {
	counter := 0
	for _, bs := range bootstrap {
		ma, err := multiaddr.NewMultiaddr(bs)
		if err != nil {
			Out.Msg("local", true, "Failed to parse bootstrap address: %v", err)
			return
		}
		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			Out.Msg("local", true, "Failed to parse bootstrap address: %v", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := node.Connect(ctx, *pi); err != nil {
			Out.Msg("local", true, "Failed to connect: %v", err)
		} else {
			counter++
		}
	}

	Out.Msg("local", true, "Bootstraping DHT from %d peers", counter)
}

var forceRejoin = make(chan struct{}, 1)

func announceTick() {
	discover := discovery.NewRoutingDiscovery(kDHT)

	t := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-forceRejoin:
			rejoinAll(discover, true)
		case <-t.C:
			rejoinAll(discover, false)
		}
	}
}

func inputLoop(quit chan<- struct{}) {
	defer close(quit)
	defer Out.Close()
	for {
		l, err := Out.ReadLine()
		if err != nil {
			if err == ErrInterrupt {
				return
			}
			Out.Msg("local", true, "Stdin error: %v", err)
			return
		}

		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "/") {
			if currentTopic == "" {
				Out.Msg("local", true, "You shout in the empty field with noone to hear you... use /join <channel>")
				continue
			}
			send(currentTopic, t)
			continue
		}

		if err := handleCommand(t); err != nil {
			if err == ErrInterrupt {
				return
			}
			Out.Msg("local", true, "%v", err)
		}
	}
}

func pullMessages(sub *pubsub.Subscription) {
	ctx := context.Background()
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if strings.HasPrefix(err.Error(), "subscription cancelled") {
				return
			}
			Out.Msg("local", true, "Pull for %s failed: %v", sub.Topic(), err)
			continue
		}

		if string(msg.GetFrom()) == string(node.ID()) {
			continue
		}

		topicName := "#" + strings.TrimPrefix(sub.Topic(), chanPrefix)

		if currentTopic == sub.Topic() {
			Out.Msg(msg.GetFrom().String(), false, "%s", string(msg.Data))
		} else {
			Out.Msg(msg.GetFrom().String()+" on "+topicName, false, "%s", string(msg.Data))
		}
	}
}
