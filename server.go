package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/PatilHrushikesh/DistributedFileSystem/p2p"
)

type FileServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	BoostrapNodes     []string
	// TCPTransportOps   p2p.TCPTransportOps
}

type FileServer struct {
	FileServerOpts

	peerlock sync.Mutex
	peers    map[string]p2p.Peer

	store  *Store
	quitch chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}
	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

// type DataMessage struct {
// 	Key  string
// 	Data []byte
// }

type MessageStoreFile struct {
	Key  string
	Size int64
}
type Message struct {
	Payload any
}

func (s *FileServer) stream(msg Message) error {
	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)
	return gob.NewEncoder(mw).Encode(msg)
}

func (s *FileServer) broadcast(msg *Message) error {
	buf := new(bytes.Buffer)
	fmt.Printf("Encoding msg: %+v\n", msg)
	if err := gob.NewEncoder(buf).Encode(&msg); err != nil {
		fmt.Printf("Encoding error: %s\n", err)
		return err
	}
	// fmt.Printf("Sending payload: %s\n", buf)

	for port, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingMessage})
		fmt.Printf("Port:%s, Sending from %s to %s\n", port, peer.LocalAddr(), peer.RemoteAddr())
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

type MessageGetFile struct {
	Key string
}

func (s *FileServer) Get(key string) (io.Reader, error) {
	if s.store.Has(key) {
		return s.store.Read(key)
	}

	fmt.Printf("dont have file (%s) locally, fetching from network....\n", key)

	msg := Message{
		Payload: MessageGetFile{
			Key: key,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 30)
	for _, peer := range s.peers {
		fmt.Printf("receiving stream from peer: %s", peer.RemoteAddr())
		fileBuffer := new(bytes.Buffer)
		n, err := io.CopyN(fileBuffer, peer, 22)
		if err != nil {
			return nil, err
		}

		fmt.Println("recieved bytes over the network ", n)
		fmt.Printf("fileBuffer: %s", fileBuffer.String())
	}
	select {}
	return nil, nil
}

func (s *FileServer) Store(key string, r io.Reader) error {
	// 1. store file to disc
	// 2. broadcast this file to all peers in network
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	size, err := s.store.Write(key, tee)
	if err != nil {
		return err
	}

	msg := Message{
		Payload: MessageStoreFile{
			Key:  key,
			Size: size,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 10)
	// TODO: Use multiwriter here
	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingStream})
		n, err := io.Copy(peer, fileBuffer)
		if err != nil {
			return err
		}
		fmt.Printf("received and written bytes to disk: %d\n", n)
	}

	return nil
	// buf := new(bytes.Buffer)
	// tee := io.TeeReader(r, buf)

	// if err := s.store.Write(key, tee); err != nil {
	// 	return err
	// }

	// p := &DataMessage{
	// 	Key:  key,
	// 	Data: buf.Bytes(),
	// }

	// fmt.Printf("Stored in file: %s\n", buf.Bytes())

	// msg := &Message{
	// 	From:    "TODO",
	// 	Payload: p,
	// }
	// return s.broadcast(*msg)

}

func (s *FileServer) Stop() {
	close(s.quitch)
}

func (s *FileServer) OnPeer(p p2p.Peer) error {
	s.peerlock.Lock()
	defer s.peerlock.Unlock()

	s.peers[p.RemoteAddr().String()] = p
	fmt.Println("Key-Value Pairs:")
	for key, value := range s.peers {
		fmt.Printf("%s: %+v\n", key, value)
	}
	fmt.Println("\n")
	log.Printf("connected with remote %s\n", p.RemoteAddr())

	return nil
}

func (s *FileServer) loop() {

	defer func() {
		log.Println("file server stopped due to error or user quit")

	}()
	// time.Sleep(time.Second * 2)
	fmt.Println("Inside THE loop")
	for {
		select {
		case rpc := <-s.Transport.Consume():
			var msg Message

			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Printf("error while decoding %s", err)
				return
			}

			if err := s.handleMessage(rpc.From.String(), &msg); err != nil {
				log.Println("handle message error: ", err)
				return
			}
			// fmt.Printf("msg.payload : %+v\n", msg)
			// peer, ok := s.peers[rpc.From.String()]
			// if !ok {
			// 	panic("peer not found in peer map")
			// }
			// fmt.Printf("recv to %s from :%s\n", peer.LocalAddr(), peer.RemoteAddr())

			// b := make([]byte, 10000)
			// if _, err := peer.Read(b); err != nil {
			// 	panic(err)
			// }

			// fmt.Printf("peer2: %s\n", string(b))
			// peer.(*p2p.TCPPeer).Wg.Done()
		case <-s.quitch:
			return

		}
	}

}

func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, v)
	case MessageGetFile:
		return s.handleMessageGetFile(from, v)
	}
	return nil
}

func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !s.store.Has(msg.Key) {
		return fmt.Errorf("need to serve file (%s) does not exist on disk", msg.Key)
	}

	fmt.Printf("serving file (%s) over the network\n", msg.Key)

	r, err := s.store.Read(msg.Key)
	if err != nil {
		return err
	}

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not in map", from)
	}

	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	fmt.Printf("written %d butes over the netwoek to %s\n", n, from)
	return nil
}

func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	// fmt.Printf("recv store file:%+v\n", msg)
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer (%s) cound not be found", from)
	}

	n, err := s.store.Write(msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}
	fmt.Printf("[%s] written %d bytes to disk\n", s.Transport.Addr(), n)
	peer.CloseStream()
	return nil

}

func (s *FileServer) BoostrapNetwork() error {
	for _, addr := range s.BoostrapNodes {
		go func(addr string) {
			fmt.Printf("attempting to connect with remote: %s\n", addr)
			if err := s.Transport.Dial(addr); err != nil {
				log.Println("dial error: ", err)
			}
		}(addr)
	}
	return nil
}

func (s *FileServer) Start() error {
	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}
	s.BoostrapNetwork()
	s.loop()

	return nil
}

func init() {
	// fmt.Println("Registering GOBencoder")
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}
