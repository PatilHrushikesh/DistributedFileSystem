package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/PatilHrushikesh/DistributedFileSystem/p2p"
)

func OnPeer(p2p.Peer) error {
	fmt.Println("doing some logic with the peer outside TCPTransport")
	return nil
}

func makeServer(listenAddr string, nodes ...string) *FileServer {
	tcptransportOps := p2p.TCPTransportOps{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		// OnPeer:        p2p.OnPeer, // We need to have server first?
	}
	tcpTransport := p2p.NewTCPTransport(tcptransportOps)
	fmt.Printf("tcpTransport : %+v", tcpTransport)
	fileServerOpts := FileServerOpts{
		StorageRoot:       listenAddr + "_network",
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcpTransport,
		BoostrapNodes:     nodes,
	}

	s := NewFileServer(fileServerOpts)
	tcpTransport.OnPeer = s.OnPeer
	return s
}

func main() {

	s1 := makeServer(":3000")
	s2 := makeServer(":4000", ":3000")
	fmt.Printf("s2 :%++v", s2.Transport)

	go func() {
		log.Fatal(s1.Start())
	}()
	time.Sleep(1 * time.Second)
	// fmt.Println("=====")

	go s2.Start()
	time.Sleep(1 * time.Second)

	for i := 0; i < 10; i++ {
		data := bytes.NewReader([]byte("my big file data here2"))
		if err := s2.Store(fmt.Sprintf("mypriatekeyHRushi_%d", i), data); err != nil {
			log.Fatal("Error while storing data", err)
		}
		time.Sleep(time.Millisecond * 50)
	}

	// r, err := s2.Get("mypriatekeyHRushi2")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// b, err := io.ReadAll(r)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(string(b))

	select {}
}
