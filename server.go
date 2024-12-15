package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/PatilHrushikesh/DistributedFileSystem/p2p"
)

type FileServerOpts struct {
	EncKey            []byte
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
		fmt.Printf("[%s] serving file (%s) locally....\n", s.Transport.Addr(), key)
		_, r, err := s.store.Read(key)
		return r, err
	}

	fmt.Printf("[%s] dont have file (%s) locally, fetching from network....\n", s.Transport.Addr(), key)

	msg := Message{
		Payload: MessageGetFile{
			Key: hashKey(key),
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 500)

	for _, peer := range s.peers {
		//  First read file size so we can limit the amount of bytes
		//  that we can read from connection, so it will not keep hanging
		fmt.Printf("receiving stream from peer: %s\n", peer.RemoteAddr())

		var fileSize int64
		binary.Read(peer, binary.LittleEndian, &fileSize)
		// n, err := s.store.Write(key, io.LimitReader(peer, fileSize))
		n, err := s.store.WriteDecrypt(s.EncKey, key, io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, err
		}

		fmt.Printf("[%s] recieved (%d) bytes from the network (%s)\n", s.Transport.Addr(), n, peer.RemoteAddr())

		peer.CloseStream()
	}
	_, r, err := s.store.Read(key)
	return r, err
}

func (s *FileServer) Store(key string, r io.Reader) error {
	// 1. store file to disc
	// 2. broadcast this file to all peers in network
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	size, err := s.store.Write(key, tee)
	// n, err := copyEncrypt(s.EncKey, fileBuffer, tee)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}

	msg := Message{
		Payload: MessageStoreFile{
			Key:  hashKey(key),
			Size: size + 16,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 10)
	fmt.Printf("[STORE] Sending\n")

	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)
	mw.Write([]byte{p2p.IncomingStream})
	n, err := copyEncrypt(s.EncKey, fileBuffer, mw)
	if err != nil {
		return err
	}
	fmt.Printf("[%s] received and written (%d) bytes to disk\n", s.Transport.Addr(), n)

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
		return fmt.Errorf("[%s] need to serve file (%s) does not exist on disk", s.Transport.Addr(), msg.Key)
	}

	fmt.Printf("serving file (%s) over the network\n", msg.Key)

	fileSize, r, err := s.store.Read(msg.Key)
	if err != nil {
		return err
	}

	rc, ok := r.(io.ReadCloser)
	if ok {
		fmt.Println("[handleGet] closing readCloser")
		defer rc.Close()
	}

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not in map", from)
	}

	peer.Send([]byte{p2p.IncomingStream})

	binary.Write(peer, binary.LittleEndian, fileSize)
	// n, err := io.Copy(peer, r)
	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written %d bytes over the netwoek to %s\n", s.Transport.Addr(), n, from)
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
