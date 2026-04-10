package stellar

import (
	"crypto/tls"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

// AutoCert returns a *tls.Config that fetches and renews TLS certificates
// automatically from Let's Encrypt for the given domain. Certificates are
// cached on disk in cacheDir. Also starts the HTTP-01 challenge listener on
// :80 in the background, which Let's Encrypt requires to verify domain ownership.
func AutoCert(domain, cacheDir string) *tls.Config {
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
		Cache:      autocert.DirCache(cacheDir),
	}
	go http.ListenAndServe(":80", m.HTTPHandler(nil)) //nolint:errcheck
	return m.TLSConfig()
}

// ManualCert returns a *tls.Config pre-loaded with the cert/key pair at the
// given paths. Suitable for self-signed certs in development or certs issued
// by a CA and managed externally.
func ManualCert(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}
