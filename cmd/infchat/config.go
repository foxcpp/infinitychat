package main

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	PrivateKeyPath string `toml:"private_key_path"`

	Swarm struct {
		Bootstrap []string `toml:"bootstrap"`
		PSK       string   `toml:"psk"`

		ListenAddrs  []string `toml:"listen_addrs"`
		StaticRelays []string `toml:"static_relays"`

		HighWaterMark int `toml:"conns_high_watermark"`
		LowWaterMark  int `toml:"conns_low_watermark"`
	} `toml:"swarm"`

	Discovery struct {
		MDNS             bool `toml:"mdns"`
		MDNSIntervalSecs int  `toml:"mdns_interval_secs"`
	} `toml:"discovery"`

	Channels struct {
		RejoinIntervalSecs   int `toml:"rejoin_interval_secs"`
		AnnounceIntervalSecs int `toml:"Announce_interval_secs"`
	} `toml:"channels"`
}

func CreateDefaults() *Config {
	cfg := new(Config)
	cfg.PrivateKeyPath = "infinitychat.key"
	cfg.Swarm.Bootstrap = []string{
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",

		"/ip4/51.15.110.221/tcp/4001/ipfs/QmZBXSZw6qwBhqiiv6xSJJQyrC6neyz3BTjGdyTa9sovKt",
		"/ip6/2001:bc8:1840:724::1/tcp/4001/ipfs/QmZBXSZw6qwBhqiiv6xSJJQyrC6neyz3BTjGdyTa9sovKt",
	}
	cfg.Swarm.ListenAddrs = []string{
		"/ip4/0.0.0.0/tcp/18755",
		"/ip6/::/tcp/18755",
		"/ip4/0.0.0.0/udp/18755/quic",
		"/ip6/::/udp/18755/quic",
	}
	cfg.Swarm.HighWaterMark = 500
	cfg.Swarm.LowWaterMark = 50
	cfg.Discovery.MDNS = true
	cfg.Discovery.MDNSIntervalSecs = 10
	cfg.Channels.RejoinIntervalSecs = 30
	cfg.Channels.AnnounceIntervalSecs = 5 * 60 /* 5 mins */

	return cfg
}

func ReadConfig(path string) (*Config, error) {
	cfg := CreateDefaults()
	if path == "" {
		return cfg, nil
	}
	meta, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("config: read: %w", err)
	}
	for _, k := range meta.Undecoded() {
		return nil, fmt.Errorf("config: unknown key: %v", k)
	}
	return cfg, nil
}
