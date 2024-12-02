package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// TCPPeer represents the remote node over tcp established connection
type TCPPeer struct {
	// conn is underlaying connection of peer, which in
	// this case tcp connection
	net.Conn

	// if we dial and retrive connection => outbound == true
	outbound bool

	wg *sync.WaitGroup
}

func (p *TCPPeer) CloseStream() {
	p.wg.Done()
}

func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Write(b)
	return err
}

// // RemoteAddr implements the Peer interface
// func (p *TCPPeer) RemoteAddr() net.Addr {
// 	return p.conn.RemoteAddr()
// }

// // Close implements the Peer interface
// func (p *TCPPeer) Close() error {
// 	return p.conn.Close()
// }

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
		wg:       &sync.WaitGroup{},
	}
}

type TCPTransportOps struct {
	ListenAddr    string
	HandshakeFunc HandshakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	TCPTransportOps
	listerner net.Listener
	rpcch     chan RPC
}

func NewTCPTransport(ops TCPTransportOps) *TCPTransport {
	return &TCPTransport{
		TCPTransportOps: ops,
		rpcch:           make(chan RPC, 1024),
	}
}

func (t *TCPTransport) ListenAddrw() string {
	return t.ListenAddr
}

// Addr implements the Transport interface return the address
// the transport is accepting connections
func (t *TCPTransport) Addr() string {
	return t.ListenAddr
}

// Comsume implement transport interface, which will
// return read-only chan for reading the incoming msg
// recved from another peer
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

// Close implement transport interface
func (t *TCPTransport) Close() error {
	return t.listerner.Close()
}

// Dial implement transport interface
func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	go t.handleConn(conn, true)
	return nil
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listerner, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}
	go t.startAcceptLoop()
	log.Printf("TCP transport listening on port: %s\n", t.ListenAddr)
	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {

		conn, err := t.listerner.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}

		if err != nil {
			fmt.Printf("TCP accept error: %s\n", err)
		}

		fmt.Printf("New incoming connection to node %s from %s\n", conn.LocalAddr(), conn.RemoteAddr())
		go t.handleConn(conn, false)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {

	var err error
	defer func() {
		fmt.Printf("Dropping peer connection: %s", err)
		conn.Close()
	}()

	peer := NewTCPPeer(conn, outbound)

	if err = t.HandshakeFunc(peer); err != nil {
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	// buf := make([]byte, 2000)
	for {
		rpc := &RPC{}
		if err = t.Decoder.Decode(conn, rpc); err != nil {
			if err != nil {
				log.Fatal(err)
			}
			// if err == io.EOF {
			// 	fmt.Println("Connection closed by the peer")
			// 	return
			// }
			// fmt.Printf("TCP error: %s\n", err)
			// continue
		}
		fmt.Printf("Local addr: %s\n", conn.LocalAddr())

		rpc.From = conn.RemoteAddr()
		if rpc.Stream {
			peer.wg.Add(1)
			fmt.Printf("[%s] incoming stream, waiting ...\n", conn.RemoteAddr())
			peer.wg.Wait()
			fmt.Printf("[%s] stream closed, resuming read loop...\n", conn.RemoteAddr())
			continue
		}

		t.rpcch <- *rpc
		fmt.Print("stream done\n=======\n\n")
		// n, _ := conn.Read(buf)
		// fmt.Printf("Message: %+v\n", string(buf[:n]))

	}
}
