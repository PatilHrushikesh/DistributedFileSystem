package p2p

// var ErrInvalidHandshake = errors.New("invalid handshake")

type HandshakeFunc func(Peer) error

func NOPHandshakeFunc(Peer) error { return nil }
