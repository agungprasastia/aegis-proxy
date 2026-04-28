package mitm

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/aegis-proxy/aegis/internal/config"
)

type MITMProxy struct {
	cfg          *config.Config
	certManager  *CertificateManager
	hostsManager *HostsManager
	trustManager *TrustStoreManager
	server       *http.Server
	certCache    map[string]*tls.Certificate
	cacheMu      sync.RWMutex
	upstreamAddr string
}

func NewMITMProxy(cfg *config.Config) *MITMProxy {
	certManager := NewCertificateManager(cfg.DataDir)
	return &MITMProxy{
		cfg:          cfg,
		certManager:  certManager,
		hostsManager: NewHostsManager(),
		trustManager: NewTrustStoreManager(certManager.GetCertPath()),
		certCache:    make(map[string]*tls.Certificate),
		upstreamAddr: fmt.Sprintf("127.0.0.1:%d", cfg.ProxyPort),
	}
}

func (mp *MITMProxy) Start() error {
	tlsConfig := &tls.Config{
		GetCertificate: mp.getCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	mp.server = &http.Server{
		Addr:      ":8443",
		Handler:   mp,
		TLSConfig: tlsConfig,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting MITM proxy on :8443")
	return mp.server.ListenAndServeTLS("", "")
}

func (mp *MITMProxy) Shutdown(ctx context.Context) error {
	if mp.server != nil {
		return mp.server.Shutdown(ctx)
	}
	return nil
}

func (mp *MITMProxy) SetupCA() error {
	if mp.certManager.CertExists() {
		log.Println("CA certificate already exists, loading...")
		return mp.certManager.LoadCert()
	}

	log.Println("Generating new CA certificate...")
	return mp.certManager.GenerateCA()
}

func (mp *MITMProxy) SetupHosts() error {
	domains := []string{
		"api.openai.com",
		"api.anthropic.com",
		"api.cohere.ai",
		"generativelanguage.googleapis.com",
	}

	log.Println("Adding entries to hosts file...")
	for _, domain := range domains {
		if err := mp.hostsManager.AddEntry(domain, "127.0.0.1"); err != nil {
			return fmt.Errorf("failed to add hosts entry for %s: %w", domain, err)
		}
	}

	return nil
}

func (mp *MITMProxy) SetupTrustStore() error {
	if mp.trustManager.IsCertInstalled() {
		log.Println("CA certificate already installed in trust store")
		return nil
	}

	log.Println("Installing CA certificate to system trust store...")
	return mp.trustManager.InstallCert(mp.certManager.GetCertPath())
}

func (mp *MITMProxy) Cleanup() error {
	log.Println("Cleaning up MITM proxy setup...")

	if err := mp.hostsManager.Cleanup(); err != nil {
		log.Printf("Warning: failed to cleanup hosts file: %v", err)
	}

	if err := mp.trustManager.UninstallCert(); err != nil {
		log.Printf("Warning: failed to uninstall certificate: %v", err)
	}

	return nil
}

func (mp *MITMProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		mp.handleConnect(w, r)
		return
	}

	mp.proxyRequest(w, r)
}

func (mp *MITMProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	cert, err := mp.getCertForHost(host)
	if err != nil {
		log.Printf("Failed to get certificate for %s: %v", host, err)
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	tlsConn := tls.Server(clientConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake failed: %v", err)
		return
	}
	defer tlsConn.Close()

	reader := bufio.NewReader(tlsConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("Failed to read request: %v", err)
		return
	}

	req.URL.Scheme = "https"
	req.URL.Host = r.Host
	req.RequestURI = ""

	mp.forwardToUpstream(tlsConn, req)
}

func (mp *MITMProxy) proxyRequest(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(nil),
		},
	}

	proxyReq, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (mp *MITMProxy) forwardToUpstream(conn net.Conn, req *http.Request) {
	upstreamConn, err := net.Dial("tcp", mp.upstreamAddr)
	if err != nil {
		log.Printf("Failed to connect to upstream: %v", err)
		return
	}
	defer upstreamConn.Close()

	if err := req.Write(upstreamConn); err != nil {
		log.Printf("Failed to write request to upstream: %v", err)
		return
	}

	done := make(chan struct{})
	go func() {
		io.Copy(conn, upstreamConn)
		done <- struct{}{}
	}()

	io.Copy(upstreamConn, conn)
	<-done
}

func (mp *MITMProxy) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return mp.getCertForHost(hello.ServerName)
}

func (mp *MITMProxy) getCertForHost(host string) (*tls.Certificate, error) {
	mp.cacheMu.RLock()
	if cert, ok := mp.certCache[host]; ok {
		mp.cacheMu.RUnlock()
		return cert, nil
	}
	mp.cacheMu.RUnlock()

	certPEM, keyPEM, err := mp.certManager.GenerateCert(host)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	mp.cacheMu.Lock()
	mp.certCache[host] = &cert
	mp.cacheMu.Unlock()

	return &cert, nil
}
