package tunnel

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/eliot-lemaire/proxy-centauri/internal/balancer"
)

// udpSession tracks a single client's sticky backend connection.
type udpSession struct {
	backendConn net.Conn
	lastSeen    time.Time
	mu          sync.Mutex
}

// UDPTunnel is a stateless-protocol proxy. It accepts datagrams on one port
// and forwards them to a backend Star System, maintaining per-client sticky
// sessions so replies route back to the correct client.
type UDPTunnel struct {
	balancer balancer.Balancer
	sessions sync.Map // clientAddr string → *udpSession
}

// NewUDP creates a UDPTunnel backed by the given balancer.
func NewUDP(lb balancer.Balancer) *UDPTunnel {
	return &UDPTunnel{balancer: lb}
}

// Listen opens a UDP socket on address and processes datagrams forever.
// One goroutine handles all client sessions; a background goroutine evicts
// sessions idle for more than 30 seconds.
func (t *UDPTunnel) Listen(address string) {
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Printf("  [ UDP Tunnel ] failed to listen on %s: %v", address, err)
		return
	}
	defer conn.Close()

	go t.evict(conn, 30*time.Second)

	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("  [ UDP Tunnel ] read error on %s: %v", address, err)
			return
		}

		data := make([]byte, n)
		copy(data, buf[:n])
		go t.forward(conn, clientAddr, data)
	}
}

// forward routes one datagram from clientAddr to its sticky backend session,
// creating a new session (and backend goroutine) if none exists yet.
func (t *UDPTunnel) forward(conn net.PacketConn, clientAddr net.Addr, data []byte) {
	key := clientAddr.String()

	v, loaded := t.sessions.Load(key)
	if !loaded {
		addr := t.balancer.Next()
		if addr == "" {
			log.Println("  [ UDP Tunnel ] all star systems are dead — dropping datagram")
			return
		}

		bc, err := net.Dial("udp", addr)
		if err != nil {
			log.Printf("  [ UDP Tunnel ] could not reach backend %s: %v", addr, err)
			return
		}

		sess := &udpSession{backendConn: bc, lastSeen: time.Now()}
		actual, existed := t.sessions.LoadOrStore(key, sess)
		if existed {
			// Another goroutine raced and stored first — close our extra conn.
			bc.Close()
			v = actual
		} else {
			v = sess
			// Start the reply pump for this new session.
			go t.pump(conn, clientAddr, sess)
		}
	}

	sess := v.(*udpSession)
	sess.mu.Lock()
	sess.lastSeen = time.Now()
	sess.mu.Unlock()

	if _, err := sess.backendConn.Write(data); err != nil {
		log.Printf("  [ UDP Tunnel ] write to backend error: %v", err)
	}
}

// pump reads replies from the backend and sends them back to the client.
// It runs in its own goroutine for the lifetime of the session.
func (t *UDPTunnel) pump(conn net.PacketConn, clientAddr net.Addr, sess *udpSession) {
	buf := make([]byte, 65535)
	for {
		n, err := sess.backendConn.Read(buf)
		if err != nil {
			return
		}
		if _, err := conn.WriteTo(buf[:n], clientAddr); err != nil {
			return
		}
	}
}

// evict periodically closes and removes sessions that have been idle longer
// than the given duration. This prevents unbounded memory growth.
func (t *UDPTunnel) evict(_ net.PacketConn, idle time.Duration) {
	ticker := time.NewTicker(idle)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-idle)
		t.sessions.Range(func(k, v any) bool {
			sess := v.(*udpSession)
			sess.mu.Lock()
			last := sess.lastSeen
			sess.mu.Unlock()
			if last.Before(cutoff) {
				sess.backendConn.Close()
				t.sessions.Delete(k)
			}
			return true
		})
	}
}
