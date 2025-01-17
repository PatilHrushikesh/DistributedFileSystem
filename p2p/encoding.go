package p2p

import (
	"encoding/gob"
	"fmt"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type GOBDecoder struct{}

func (dec GOBDecoder) Decode(r io.Reader, msg *RPC) error {
	return gob.NewDecoder(r).Decode(msg)

}

type DefaultDecoder struct{}

func (dec DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	peekBuf := make([]byte, 1)
	fmt.Printf("waiting to read byte\n")
	if _, err := r.Read(peekBuf); err != nil {
		return nil
	}

	// In case of stream we are not decoding what is being sent over the network
	// We are jsut setting Stream true so we can handle that in our logic
	stream := peekBuf[0] == IncomingStream
	if stream {
		msg.Stream = true
		return nil
	}
	buf := make([]byte, 1028)

	fmt.Printf("Reading stream data\n")
	n, err := r.Read(buf)
	if err != nil {
		return err
	}
	msg.Payload = buf[:n]
	return nil
}
