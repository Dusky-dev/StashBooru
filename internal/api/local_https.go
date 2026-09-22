package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
)

const (
	localHTTPSCertFilename = "stash-local.crt"
	localHTTPSKeyFilename  = "stash-local.key"
)

type localHTTPSServerState struct {
	server *http.Server
}

var localHTTPSServers sync.Map

// StartLocalHTTPS starts a secondary HTTPS listener for normal HTTP installs.
// Existing explicit Stash TLS configurations retain their legacy single-port
// behavior and therefore do not need the secondary listener.
func (s *Server) StartLocalHTTPS() error {
	cfg := config.GetInstance()
	if !cfg.GetLocalHTTPSEnabled() || s.TLSConfig != nil {
		return nil
	}

	port := cfg.GetLocalHTTPSPort()
	if port == cfg.GetPort() {
		return fmt.Errorf("local HTTPS port %d conflicts with the HTTP port", port)
	}

	certFile, keyFile, generated, hosts, err := localHTTPSCertificateFiles(cfg)
	if err != nil {
		return err
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("loading local HTTPS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	address := net.JoinHostPort(cfg.GetHost(), strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listening for local HTTPS on %s: %w", address, err)
	}

	httpsServer := &http.Server{
		Addr:      address,
		Handler:   s.Handler,
		TLSConfig: tlsConfig,
		// Match the primary server: HTTP/2 is disabled because streaming
		// endpoints rely on connection hijacking when media is deleted.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	localHTTPSServers.Store(s, localHTTPSServerState{server: httpsServer})

	displayHost := localHTTPSDisplayHost(cfg)
	logger.Infof("stash local HTTPS is listening on %s", address)
	logger.Infof("stash local HTTPS is available at https://%s/", net.JoinHostPort(displayHost, strconv.Itoa(port)))
	if generated {
		logger.Infof("using persistent self-signed local HTTPS certificate %s", certFile)
		logger.Warn("local HTTPS uses a self-signed certificate by default; trust it on your client to remove browser certificate warnings")
		if runningInContainer() && len(cfg.GetLocalHTTPSHosts()) == 0 {
			logger.Warn("Docker cannot discover the host LAN address from bridge networking; set STASH_LOCAL_HTTPS_HOSTS to the LAN IP/hostname if you want it included in the certificate SANs")
		}
	}
	logger.Debugf("local HTTPS certificate hosts: %s", strings.Join(hosts, ", "))

	go func() {
		err := httpsServer.Serve(tls.NewListener(listener, tlsConfig))
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("local HTTPS server error: %v", err)
		}
	}()

	return nil
}

// ShutdownLocalHTTPS gracefully stops the secondary HTTPS listener, if active.
func (s *Server) ShutdownLocalHTTPS() {
	stateValue, ok := localHTTPSServers.LoadAndDelete(s)
	if !ok {
		return
	}

	state := stateValue.(localHTTPSServerState)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := state.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Errorf("error shutting down local HTTPS server: %v", err)
	}
}

func localHTTPSCertificateFiles(cfg *config.Config) (certFile, keyFile string, generated bool, hosts []string, err error) {
	certFile = strings.TrimSpace(cfg.GetLocalHTTPSCertPath())
	keyFile = strings.TrimSpace(cfg.GetLocalHTTPSKeyPath())

	if (certFile == "") != (keyFile == "") {
		return "", "", false, nil, errors.New("local HTTPS certificate and key paths must both be configured")
	}

	hosts = localHTTPSCertificateHosts(cfg)
	if certFile != "" {
		return certFile, keyFile, false, hosts, nil
	}

	certFile = filepath.Join(cfg.GetConfigPath(), localHTTPSCertFilename)
	keyFile = filepath.Join(cfg.GetConfigPath(), localHTTPSKeyFilename)
	if localCertificateMatches(certFile, keyFile, hosts) {
		return certFile, keyFile, true, hosts, nil
	}

	if err := generateLocalCertificate(certFile, keyFile, hosts); err != nil {
		return "", "", false, nil, fmt.Errorf("generating local HTTPS certificate: %w", err)
	}

	return certFile, keyFile, true, hosts, nil
}

func localHTTPSCertificateHosts(cfg *config.Config) []string {
	hosts := map[string]struct{}{}
	addCertificateHost(hosts, "localhost")
	addCertificateHost(hosts, "127.0.0.1")
	addCertificateHost(hosts, "::1")
	addCertificateHost(hosts, "stash")

	configuredHost := cfg.GetHost()
	if configuredHost != "" && configuredHost != "0.0.0.0" && configuredHost != "::" {
		addCertificateHost(hosts, configuredHost)
	}

	if externalHost := strings.TrimSpace(cfg.GetExternalHost()); externalHost != "" {
		if parsed, err := url.Parse(externalHost); err == nil && parsed.Hostname() != "" {
			addCertificateHost(hosts, parsed.Hostname())
		}
	}

	for _, host := range cfg.GetLocalHTTPSHosts() {
		addCertificateHost(hosts, host)
	}

	// On bare-metal installs the process can see the real LAN interfaces, so add
	// those automatically. Docker bridge networking only exposes container-side
	// addresses; those are deliberately skipped because they do not match the
	// host address clients use to reach the published port.
	if !runningInContainer() {
		if hostname, err := os.Hostname(); err == nil {
			addCertificateHost(hosts, hostname)
		}
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, addr := range addrs {
				var ip net.IP
				switch value := addr.(type) {
				case *net.IPNet:
					ip = value.IP
				case *net.IPAddr:
					ip = value.IP
				}
				if ip != nil && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast() {
					addCertificateHost(hosts, ip.String())
				}
			}
		}
	}

	ret := make([]string, 0, len(hosts))
	for host := range hosts {
		ret = append(ret, host)
	}
	sort.Strings(ret)
	return ret
}

func addCertificateHost(hosts map[string]struct{}, host string) {
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}

	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
	} else if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		return
	}
	hosts[host] = struct{}{}
}

func localHTTPSDisplayHost(cfg *config.Config) string {
	if hosts := cfg.GetLocalHTTPSHosts(); len(hosts) > 0 {
		hostsSet := map[string]struct{}{}
		addCertificateHost(hostsSet, hosts[0])
		for host := range hostsSet {
			return host
		}
	}

	host := cfg.GetHost()
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "localhost"
	}
	return host
}

func runningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		text := string(data)
		return strings.Contains(text, "docker") || strings.Contains(text, "containerd") || strings.Contains(text, "kubepods")
	}
	return false
}

func localCertificateMatches(certFile, keyFile string, hosts []string) bool {
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil || len(pair.Certificate) == 0 {
		return false
	}

	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return false
	}

	now := time.Now()
	if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter.Add(-24*time.Hour)) {
		return false
	}

	for _, host := range hosts {
		if err := cert.VerifyHostname(host); err != nil {
			return false
		}
	}
	return true
}

func generateLocalCertificate(certFile, keyFile string, hosts []string) error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"StashBooru"},
			CommonName:   "StashBooru Local HTTPS",
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	if err := os.MkdirAll(filepath.Dir(certFile), 0700); err != nil {
		return err
	}
	if filepath.Dir(keyFile) != filepath.Dir(certFile) {
		if err := os.MkdirAll(filepath.Dir(keyFile), 0700); err != nil {
			return err
		}
	}

	// Write the key first. If the process is interrupted before the certificate
	// rename, the pair simply fails validation and is regenerated on next start.
	if err := writeLocalHTTPSFile(keyFile, keyPEM, 0600); err != nil {
		return err
	}
	if err := writeLocalHTTPSFile(certFile, certPEM, 0644); err != nil {
		return err
	}
	return nil
}

func writeLocalHTTPSFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".stash-local-https-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}

	// Windows does not replace an existing destination with os.Rename. Remove
	// only that known destination and retry, preserving atomic replacement on
	// platforms where rename-over-existing is supported.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpName, path)
}
