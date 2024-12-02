package p2p

import "net"

// Peer represent remote node
type Peer interface {
	net.Conn
	Send([]byte) error
	CloseStream()
}

type Transport interface {
	Addr() string
	Dial(string) error
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
}
