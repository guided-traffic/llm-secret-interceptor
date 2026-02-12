package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCA(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "llm-proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	// Generate CA
	err = GenerateCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Error("Certificate file not created")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file not created")
	}

	// Verify certificate is valid
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to read certificate: %v", err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key: %v", err)
	}

	// Parse as TLS certificate
	_, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
}

func TestCertManager(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "llm-proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	// Generate CA
	err = GenerateCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Create CertManager
	cm, err := NewCertManager(certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertManager failed: %v", err)
	}

	// Test certificate generation
	testCases := []struct {
		hostname string
	}{
		{"example.com"},
		{"api.openai.com"},
		{"192.168.1.1"},
		{"localhost"},
	}

	for _, tc := range testCases {
		t.Run(tc.hostname, func(t *testing.T) {
			hello := &tls.ClientHelloInfo{ServerName: tc.hostname}
			cert, err := cm.GetCertificate(hello)
			if err != nil {
				t.Fatalf("GetCertificate failed: %v", err)
			}
			if cert == nil {
				t.Fatal("Certificate is nil")
			}

			// Verify certificate
			x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				t.Fatalf("Failed to parse x509 certificate: %v", err)
			}

			// Check common name or SAN
			if x509Cert.Subject.CommonName != tc.hostname {
				t.Errorf("CommonName mismatch: got %s, want %s", x509Cert.Subject.CommonName, tc.hostname)
			}
		})
	}
}

func TestCertManagerCaching(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "llm-proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	// Generate CA
	err = GenerateCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Create CertManager
	cm, err := NewCertManager(certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertManager failed: %v", err)
	}

	// Get certificate twice
	hello := &tls.ClientHelloInfo{ServerName: "example.com"}
	cert1, err := cm.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}

	cert2, err := cm.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}

	// Should be the same (cached)
	if cert1 != cert2 {
		t.Error("Certificates should be cached and identical")
	}
}

func TestGetCACertificate(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "llm-proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	// Generate CA
	err = GenerateCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Create CertManager
	cm, err := NewCertManager(certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertManager failed: %v", err)
	}

	// Get CA certificate
	caCert := cm.GetCACertificate()
	if len(caCert) == 0 {
		t.Error("CA certificate is empty")
	}

	// Should start with PEM header
	expectedHeader := "-----BEGIN CERTIFICATE-----"
	if string(caCert[:len(expectedHeader)]) != expectedHeader {
		t.Error("CA certificate is not in PEM format")
	}
}
