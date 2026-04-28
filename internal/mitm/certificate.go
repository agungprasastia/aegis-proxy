package mitm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type CertificateManager struct {
	certDir  string
	caPath   string
	keyPath  string
	caCert   *x509.Certificate
	caKey    *rsa.PrivateKey
}

func NewCertificateManager(dataDir string) *CertificateManager {
	certDir := filepath.Join(dataDir, "certs")
	return &CertificateManager{
		certDir: certDir,
		caPath:  filepath.Join(certDir, "ca.crt"),
		keyPath: filepath.Join(certDir, "ca.key"),
	}
}

func (cm *CertificateManager) GenerateCA() error {
	if err := os.MkdirAll(cm.certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Aegis Proxy CA"},
			CommonName:   "Aegis Proxy Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	if err := cm.SaveCert(certPEM, keyPEM); err != nil {
		return err
	}

	cm.caCert = &template
	cm.caKey = priv

	return nil
}

func (cm *CertificateManager) GenerateCert(domain string) ([]byte, []byte, error) {
	if cm.caCert == nil || cm.caKey == nil {
		if err := cm.LoadCert(); err != nil {
			return nil, nil, fmt.Errorf("CA not loaded: %w", err)
		}
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Aegis Proxy"},
			CommonName:   domain,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{domain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, cm.caCert, &priv.PublicKey, cm.caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPEM, keyPEM, nil
}

func (cm *CertificateManager) GetCertPath() string {
	return cm.caPath
}

func (cm *CertificateManager) GetKeyPath() string {
	return cm.keyPath
}

func (cm *CertificateManager) SaveCert(cert, key []byte) error {
	if err := os.WriteFile(cm.caPath, cert, 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	if err := os.WriteFile(cm.keyPath, key, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}

func (cm *CertificateManager) LoadCert() error {
	certPEM, err := os.ReadFile(cm.caPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cm.GenerateCA()
		}
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	keyPEM, err := os.ReadFile(cm.keyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode private key PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	cm.caCert = cert
	cm.caKey = key

	return nil
}

func (cm *CertificateManager) CertExists() bool {
	_, err := os.Stat(cm.caPath)
	return err == nil
}
