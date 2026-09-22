package api

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGenerateLocalCertificateIncludesDNSAndIPSANs(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "stash-local.crt")
	keyFile := filepath.Join(dir, "stash-local.key")
	hosts := []string{"127.0.0.1", "192.168.1.50", "localhost", "stash.test"}

	if err := generateLocalCertificate(certFile, keyFile, hosts); err != nil {
		t.Fatalf("generateLocalCertificate: %v", err)
	}
	if !localCertificateMatches(certFile, keyFile, hosts) {
		t.Fatal("generated certificate does not match requested hosts")
	}

	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("LoadX509KeyPair: %v", err)
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}

	for _, host := range hosts {
		if err := cert.VerifyHostname(host); err != nil {
			t.Errorf("certificate does not verify %q: %v", host, err)
		}
	}

	if runtime.GOOS != "windows" {
		keyInfo, err := os.Stat(keyFile)
		if err != nil {
			t.Fatalf("stat key: %v", err)
		}
		if got := keyInfo.Mode().Perm(); got != 0600 {
			t.Errorf("key permissions = %o, want 600", got)
		}
	}
}

func TestLocalCertificateMatchesRejectsMissingSAN(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "stash-local.crt")
	keyFile := filepath.Join(dir, "stash-local.key")

	if err := generateLocalCertificate(certFile, keyFile, []string{"localhost"}); err != nil {
		t.Fatalf("generateLocalCertificate: %v", err)
	}
	if localCertificateMatches(certFile, keyFile, []string{"localhost", "192.168.1.50"}) {
		t.Fatal("certificate unexpectedly matched a host absent from its SANs")
	}
}

func TestAddCertificateHostNormalizesURLsAndSocketAddresses(t *testing.T) {
	hosts := map[string]struct{}{}
	addCertificateHost(hosts, "https://stash.example:9443/path")
	addCertificateHost(hosts, "192.168.1.50:9443")
	addCertificateHost(hosts, "[fd00::10]:9443")

	for _, want := range []string{"stash.example", "192.168.1.50", "fd00::10"} {
		if _, ok := hosts[want]; !ok {
			t.Errorf("normalized hosts missing %q: %#v", want, hosts)
		}
	}

	if ip := net.ParseIP("fd00::10"); ip == nil {
		t.Fatal("test IPv6 address is invalid")
	}
}
