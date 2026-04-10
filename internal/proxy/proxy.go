package proxy

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/eliot-lemaire/proxy-centauri/internal/balancer"
)

// backendAddrKey is the context key used to pass the selected backend address
// from the director to ModifyResponse/ErrorHandler for LeastConn release.
type backendAddrKey struct{}

// Proxy is an HTTP reverse proxy backed by a load balancer.
// It forwards each incoming request to the next live Star System.
type Proxy struct {
	balancer balancer.Balancer
	rp       *httputil.ReverseProxy
}

// New creates a Proxy that uses lb to select a backend for each request.
func New(lb balancer.Balancer) *Proxy {
	p := &Proxy{balancer: lb}

	p.rp = &httputil.ReverseProxy{
		Director: p.director,
		ModifyResponse: func(resp *http.Response) error {
			if lc, ok := p.balancer.(*balancer.LeastConn); ok {
				if t, ok := resp.Request.Context().Value(backendAddrKey{}).(*string); ok && *t != "" {
					lc.Release(*t)
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if lc, ok := p.balancer.(*balancer.LeastConn); ok {
				if t, ok := r.Context().Value(backendAddrKey{}).(*string); ok && *t != "" {
					lc.Release(*t)
				}
			}
			log.Printf("  [ Jump Gate ] backend error: %v", err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		},
	}

	return p
}

// ServeHTTP satisfies http.Handler — called by the HTTP server for every request.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Inject a tracking pointer so director can pass the selected backend addr
	// back to ModifyResponse / ErrorHandler for LeastConn release.
	tracking := new(string)
	r = r.WithContext(context.WithValue(r.Context(), backendAddrKey{}, tracking))
	p.rp.ServeHTTP(w, r)
}

// director rewrites the incoming request to point at the next live backend.
// It is called by httputil.ReverseProxy before forwarding each request.
func (p *Proxy) director(req *http.Request) {
	addr := p.balancer.Next()

	// Store selected addr for LeastConn release after the request completes.
	if t, ok := req.Context().Value(backendAddrKey{}).(*string); ok {
		*t = addr
	}
	if lc, ok := p.balancer.(*balancer.LeastConn); ok && addr != "" {
		lc.Acquire(addr)
	}

	if addr == "" {
		// All Star Systems are dead — signal a 503 by setting a sentinel URL.
		// The ErrorHandler will fire when the connection fails.
		req.URL, _ = url.Parse("http://127.0.0.1:0/")
		return
	}

	target, err := url.Parse("http://" + addr)
	if err != nil {
		log.Printf("  [ Jump Gate ] invalid backend address %q: %v", addr, err)
		return
	}

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host

	// Tell the backend the real client IP.
	if clientIP := req.RemoteAddr; clientIP != "" {
		req.Header.Set("X-Forwarded-For", clientIP)
	}
}
