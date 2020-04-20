package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"

	infchat "github.com/foxcpp/infinitychat/node"
	"github.com/foxcpp/infinitychat/serialui"
	"github.com/foxcpp/infinitychat/serialui/tui"
	golog "github.com/ipfs/go-log"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
)

func loadKey(ui serialui.UI, path string) (ed25519.PrivateKey, error) {
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

		return ed25519.NewKeyFromSeed(seed), nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("loadKey: %w", err)
	}

	ui.Msg("local", true, "Generating a new ed25519 key pair...")

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("loadKey: %w", err)
	}
	if err := ioutil.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(privKey.Seed())), 0600); err != nil {
		return nil, fmt.Errorf("loadKey: %w", err)
	}

	return privKey, nil
}

func canLog() bool {
	if os.Getenv("GOLOG_FILE") != "" {
		return true
	}

	return !terminal.IsTerminal(int(os.Stderr.Fd()))
}

type RunnableUI interface {
	serialui.UI
	Run(*infchat.Node)
}

func main() {
	cfgFile := flag.String("config", "", "Configuration file to use")
	serialUI := flag.String("serialui", "tview", "Serial UI implementation to use")
	p2pLog := flag.String("libp2p-log", "warn", "libp2p logger level")
	flag.Parse()

	cfg, err := ReadConfig(*cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	var ui RunnableUI
	switch *serialUI {
	case "tview":
		ui = tui.New()
	}

	if canLog() {
		level, err := golog.LevelFromString(*p2pLog)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return
		}
		golog.SetAllLoggers(level)
	}

	key, err := loadKey(ui, cfg.PrivateKeyPath)
	if err != nil {
		ui.Error("local", true, "%v", err)
		return
	}

	node, err := infchat.NewNode(infchat.Config{
		Identity:         key,
		Bootstrap:        cfg.Swarm.Bootstrap,
		ListenAddrs:      cfg.Swarm.ListenAddrs,
		StaticRelays:     cfg.Swarm.StaticRelays,
		ConnsHigh:        cfg.Swarm.HighWaterMark,
		ConnsLow:         cfg.Swarm.LowWaterMark,
		PSK:              cfg.Swarm.PSK,
		MDNSInterval:     time.Duration(cfg.Discovery.MDNSIntervalSecs) * time.Second,
		RejoinInterval:   time.Duration(cfg.Channels.RejoinIntervalSecs) * time.Second,
		AnnounceInterval: time.Duration(cfg.Channels.AnnounceIntervalSecs) * time.Second,
		Log:              log.New(ui, "", 0),
	})
	if err != nil {
		ui.Error("local", false, "%v", err)
		return
	}
	defer node.Close()

	go serialui.InputLoop(ui, node)
	go serialui.PullMessages(ui, node)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, unix.SIGTERM, unix.SIGQUIT)

		<-sig
		ui.Close()
	}()

	go node.Run()
	ui.Run(node)
}
