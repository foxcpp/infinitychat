package main

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	PrivateKeyPath string

	Swarm struct {
		Bootstrap []string `toml:"bootstrap"`

		ListenAddrs []string `toml:"listen_addrs"`

		HighWaterMark int `toml:"conns_high_watermark"`
		LowWaterMark  int `toml:"conns_low_watermark"`
	} `toml:"p2p"`

	Channels struct {
		RejoinIntervalSecs int `toml:"rejoin_interval_secs"`
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
		"/ip4/104.236.179.241/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
		"/ip4/128.199.219.111/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",
		"/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",
		"/ip4/178.62.158.247/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",
		"/ip6/2604:a880:1:20::203:d001/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
		"/ip6/2400:6180:0:d0::151:6001/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",
		"/ip6/2604:a880:800:10::4a:5001/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",
		"/ip6/2a03:b0c0:0:1010::23:1001/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",

		"/ip4/51.15.110.221/tcp/4001/ipfs/QmZBXSZw6qwBhqiiv6xSJJQyrC6neyz3BTjGdyTa9sovKt",
		"/ip6/2001:bc8:1840:724::1/tcp/4001/ipfs/QmZBXSZw6qwBhqiiv6xSJJQyrC6neyz3BTjGdyTa9sovKt",
	}
	cfg.Swarm.ListenAddrs = []string{
		"/ip4/0.0.0.0/tcp/18755",
		"/ip6/::/tcp/18755",
		"/ip4/0.0.0.0/udp/18755/quic",
		"/ip6/::/udp/18755/quic",
	}
	cfg.Swarm.HighWaterMark = 200
	cfg.Swarm.LowWaterMark = 100
	cfg.Channels.RejoinIntervalSecs = 15

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
