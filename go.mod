module github.com/foxcpp/infinitychat

go 1.14

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/davidlazar/go-crypto v0.0.0-20190912175916-7055855a373f // indirect
	github.com/gdamore/tcell v1.3.0
	github.com/golang/protobuf v1.4.0 // indirect
	github.com/ipfs/go-log v1.0.4
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/libp2p/go-addr-util v0.0.2 // indirect
	github.com/libp2p/go-libp2p v0.8.3
	github.com/libp2p/go-libp2p-autonat v0.2.2
	github.com/libp2p/go-libp2p-connmgr v0.2.1
	github.com/libp2p/go-libp2p-core v0.5.2
	github.com/libp2p/go-libp2p-discovery v0.4.0
	github.com/libp2p/go-libp2p-kad-dht v0.7.10
	github.com/libp2p/go-libp2p-noise v0.1.0
	github.com/libp2p/go-libp2p-pubsub v0.2.7
	github.com/libp2p/go-libp2p-quic-transport v0.3.5
	github.com/libp2p/go-sockaddr v0.1.0 // indirect
	github.com/libp2p/go-yamux v1.3.6 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/miekg/dns v1.1.29 // indirect
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/multiformats/go-multibase v0.0.2 // indirect
	github.com/rivo/tview v0.0.0-20200414130344-8e06c826b3a5
	go.uber.org/zap v1.15.0 // indirect
	golang.org/x/crypto v0.0.0-20200427165652-729f1e841bcc
	golang.org/x/net v0.0.0-20200425230154-ff2c4b7c35a0 // indirect
	golang.org/x/sys v0.0.0-20200428200454-593003d681fa
	gopkg.in/irc.v3 v3.1.3
)

replace github.com/rivo/tview => github.com/foxcpp/tview v0.0.0-20200427051158-9ab7b404cc99
