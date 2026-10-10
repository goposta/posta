// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package smtprelay

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSMTPRelaySTARTTLS(t *testing.T) {
	certFile, keyFile, roots := testRelayCertificate(t)
	srv, err := NewSMTPServer(NewBackend(nil, nil, nil, 1024), SMTPConfig{
		Host: "127.0.0.1", Hostname: "localhost", TLSMode: "starttls",
		TLSCertFile: certFile, TLSKeyFile: keyFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.AllowInsecureAuth || srv.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Fatal("STARTTLS relay must require TLS before AUTH and TLS 1.2 or newer")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.Serve(listener) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		t.Fatal("STARTTLS not advertised")
	}
	if ok, _ := client.Extension("AUTH"); ok {
		t.Fatal("AUTH advertised before TLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: "localhost", RootCAs: roots, MinVersion: tls.VersionTLS12}); err != nil {
		t.Fatal(err)
	}
	if ok, mech := client.Extension("AUTH"); !ok || mech != "PLAIN" {
		t.Fatalf("AUTH after TLS = %t %q, want PLAIN", ok, mech)
	}
}

func TestSMTPRelayTLSConfiguration(t *testing.T) {
	plain, err := NewSMTPServer(nil, SMTPConfig{TLSMode: "none"})
	if err != nil || plain.TLSConfig != nil || !plain.AllowInsecureAuth {
		t.Fatalf("plaintext relay changed: server=%+v error=%v", plain, err)
	}
	for _, cfg := range []SMTPConfig{{TLSMode: "starttls"}, {TLSMode: "unexpected"}} {
		if _, err := NewSMTPServer(nil, cfg); err == nil {
			t.Fatalf("invalid TLS config %+v accepted", cfg)
		}
	}
}

func testRelayCertificate(t *testing.T) (string, string, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
	}, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
	}, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "relay.crt"), filepath.Join(dir, "relay.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certPEM)
	return certFile, keyFile, roots
}
