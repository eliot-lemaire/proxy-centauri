package stellar

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// selfSignedCert generates a self-signed ECDSA certificate and writes it to
// dir/cert.pem and dir/key.pem. Returns the paths to both files.
func selfSignedCert(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	cf, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	defer cf.Close()
	if err := pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("encode cert: %v", err)
	}

	kf, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	defer kf.Close()
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("encode key: %v", err)
	}

	return certFile, keyFile
}

// TestManualCert_Valid verifies that a valid cert/key pair produces a non-nil
// *tls.Config with exactly one certificate loaded.
func TestManualCert_Valid(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := selfSignedCert(t, dir)

	cfg, err := ManualCert(certFile, keyFile)
	if err != nil {
		t.Fatalf("ManualCert returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("ManualCert returned nil config")
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(cfg.Certificates))
	}
}

// TestManualCert_MissingFile verifies that missing files produce an error.
func TestManualCert_MissingFile(t *testing.T) {
	_, err := ManualCert("no-such.crt", "no-such.key")
	if err == nil {
		t.Fatal("expected error for missing files, got nil")
	}
}

// TestAutoCert_ReturnsConfig verifies that AutoCert returns a non-nil
// *tls.Config with GetCertificate set — no actual ACME calls are made.
func TestAutoCert_ReturnsConfig(t *testing.T) {
	cfg := AutoCert("example.com", t.TempDir())
	if cfg == nil {
		t.Fatal("AutoCert returned nil config")
	}
	if cfg.GetCertificate == nil {
		t.Fatal("AutoCert config missing GetCertificate callback")
	}
}
