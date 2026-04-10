package tunnel

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/eliot-lemaire/proxy-centauri/internal/balancer"
)

// Tunnel is a raw TCP proxy. It accepts connections on a port and pipes
// bytes bidirectionally to a backend Star System — no protocol awareness.
type Tunnel struct {
	balancer balancer.Balancer
}

// New creates a Tunnel backed by the given balancer.
func New(lb balancer.Balancer) *Tunnel {
	return &Tunnel{balancer: lb}
}

// Listen opens a TCP listener on address and accepts connections forever.
// Each connection is handled in its own goroutine.
func (t *Tunnel) Listen(address string) {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Printf("  [ TCP Tunnel ] failed to listen on %s: %v", address, err)
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("  [ TCP Tunnel ] accept error on %s: %v", address, err)
			return
		}
		go t.handle(conn)
	}
}

// handle pipes bytes between the client connection and a backend Star System.
func (t *Tunnel) handle(client net.Conn) {
	defer client.Close()

	addr := t.balancer.Next()
	if addr == "" {
		log.Println("  [ TCP Tunnel ] all star systems are dead — dropping connection")
		return
	}

	// Track active connection for LeastConn load balancing.
	if lc, ok := t.balancer.(*balancer.LeastConn); ok {
		lc.Acquire(addr)
		defer lc.Release(addr)
	}

	backend, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("  [ TCP Tunnel ] could not reach backend %s: %v", addr, err)
		return
	}
	defer backend.Close()

	// Copy bytes in both directions simultaneously.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(backend, client) // client → backend
		backend.(*net.TCPConn).CloseWrite()
	}()

	go func() {
		defer wg.Done()
		io.Copy(client, backend) // backend → client
		client.(*net.TCPConn).CloseWrite()
	}()

	wg.Wait()
}
