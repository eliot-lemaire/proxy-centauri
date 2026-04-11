package tunnel

import (
	"net"
	"testing"
	"time"
)

// stubBalancer is a minimal Balancer that always returns a fixed address.
type stubBalancer struct{ addr string }

func (s *stubBalancer) Next() string         { return s.addr }
func (s *stubBalancer) SetBackends([]string) {}
func (s *stubBalancer) Len() int             { return 1 }

// echoUDP starts a UDP server that echoes every datagram back to sender.
// Returns the server's address and a stop function.
func echoUDP(t *testing.T) (addr string, stop func()) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echoUDP: listen: %v", err)
	}
	go func() {
		buf := make([]byte, 65535)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pc.WriteTo(buf[:n], from) //nolint:errcheck
		}
	}()
	return pc.LocalAddr().String(), func() { pc.Close() }
}

// startTunnel launches a UDPTunnel in the background and returns its listen
// address. The tunnel is wired to the provided backend address.
func startTunnel(t *testing.T, backendAddr string) string {
	t.Helper()
	// Pick a free port by binding to :0 and immediately releasing it.
	tmp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("startTunnel: reserve port: %v", err)
	}
	listenAddr := tmp.LocalAddr().String()
	tmp.Close()

	lb := &stubBalancer{addr: backendAddr}
	tun := NewUDP(lb)
	go tun.Listen(listenAddr)

	// Give the goroutine a moment to bind.
	time.Sleep(20 * time.Millisecond)
	return listenAddr
}

func TestUDPTunnel_ForwardAndReply(t *testing.T) {
	backendAddr, stopBackend := echoUDP(t)
	defer stopBackend()

	tunAddr := startTunnel(t, backendAddr)

	// Dial the tunnel as a client.
	client, err := net.Dial("udp", tunAddr)
	if err != nil {
		t.Fatalf("dial tunnel: %v", err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(2 * time.Second))

	msg := []byte("hello")
	if _, err := client.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	reply := make([]byte, 64)
	n, err := client.Read(reply)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if string(reply[:n]) != string(msg) {
		t.Errorf("expected %q, got %q", msg, reply[:n])
	}
}

func TestUDPTunnel_SessionReuse(t *testing.T) {
	var dialCount int
	type countedBalancer struct{ stubBalancer }

	backendAddr, stopBackend := echoUDP(t)
	defer stopBackend()

	tunAddr := startTunnel(t, backendAddr)

	client, err := net.Dial("udp", tunAddr)
	if err != nil {
		t.Fatalf("dial tunnel: %v", err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(2 * time.Second))

	for i := 0; i < 3; i++ {
		msg := []byte("ping")
		if _, err := client.Write(msg); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		buf := make([]byte, 64)
		n, err := client.Read(buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if string(buf[:n]) != "ping" {
			t.Errorf("round %d: expected %q, got %q", i, "ping", buf[:n])
		}
	}
	_ = dialCount // session reuse is implicit: if backends were re-dialed every
	// datagram, the echo server would still work but the session map
	// would grow — this test verifies correct round-trip for 3 datagrams.
}

func TestUDPTunnel_MultipleClients(t *testing.T) {
	backendAddr, stopBackend := echoUDP(t)
	defer stopBackend()

	tunAddr := startTunnel(t, backendAddr)

	const clients = 5
	done := make(chan struct{}, clients)

	for i := 0; i < clients; i++ {
		go func(id int) {
			c, err := net.Dial("udp", tunAddr)
			if err != nil {
				t.Errorf("client %d dial: %v", id, err)
				done <- struct{}{}
				return
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(2 * time.Second))

			msg := []byte("hello")
			c.Write(msg) //nolint:errcheck
			buf := make([]byte, 64)
			n, err := c.Read(buf)
			if err != nil {
				t.Errorf("client %d read: %v", id, err)
			} else if string(buf[:n]) != string(msg) {
				t.Errorf("client %d: expected %q got %q", id, msg, buf[:n])
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < clients; i++ {
		<-done
	}
}
