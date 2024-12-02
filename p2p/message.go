package p2p

import "net"

const (
	IncomingMessage = 0x1
	IncomingStream  = 0x2
)

// RPC holds any arbitrary data send over transport between two nodes in network
type RPC struct {
	From    net.Addr
	Payload []byte
	Stream  bool
}
