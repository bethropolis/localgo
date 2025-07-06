package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"
)

// StoredSecurityContext holds PEM-encoded cert/key for config/server
// and a fingerprint for discovery. Matches Dart model.
type StoredSecurityContext struct {
	PrivateKey      string `json:"privateKey"`
	Certificate     string `json:"certificate"`
	CertificateHash string `json:"certificateHash"`
}

// GenerateKeys generates a new RSA key pair.
func generateKeys() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// encodePrivateKeyToPem encodes an RSA private key to PEM format (PKCS#1).
func encodePrivateKeyToPem(privKey *rsa.PrivateKey) string {
	privBytes := x509.MarshalPKCS1PrivateKey(privKey)
	privPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	return string(privPem)
}

// generateSelfSignedCertificate creates a self-signed X.509 certificate DER bytes.
func generateSelfSignedCertificate(privKey *rsa.PrivateKey, alias string) ([]byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"LocalSend"},
			CommonName:   alias,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, err
	}
	return certBytes, nil
}

// encodeCertificateToPem encodes certificate DER bytes to PEM format.
func encodeCertificateToPem(certBytes []byte) string {
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	return string(certPem)
}

// calculateCertificateHash calculates the SHA-256 hash of the certificate (DER format).
func calculateCertificateHash(certBytes []byte) string {
	hash := sha256.Sum256(certBytes)
	return hex.EncodeToString(hash[:])
}

// GenerateSecurityContext creates a new security context with keys and a self-signed certificate.
func GenerateSecurityContext(alias string) (*StoredSecurityContext, error) {
	privKey, err := generateKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA keys: %w", err)
	}
	certBytes, err := generateSelfSignedCertificate(privKey, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate: %w", err)
	}
	certHash := calculateCertificateHash(certBytes)
	ctx := &StoredSecurityContext{
		PrivateKey:      encodePrivateKeyToPem(privKey),
		Certificate:     encodeCertificateToPem(certBytes),
		CertificateHash: certHash,
	}
	log.Printf("Generated new Security Context. Fingerprint: %s", ctx.CertificateHash)
	return ctx, nil
}

// SaveSecurityContext saves the context as JSON to the specified path.
func SaveSecurityContext(ctx *StoredSecurityContext, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create security context file '%s': %w", path, err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(ctx); err != nil {
		return fmt.Errorf("failed to encode security context to '%s': %w", path, err)
	}
	log.Printf("Saved security context to %s", path)
	return nil
}

// LoadSecurityContext loads the context from JSON from the specified path.
func LoadSecurityContext(path string) (*StoredSecurityContext, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to open security context file '%s': %w", path, err)
	}
	defer file.Close()
	var ctx StoredSecurityContext
	if err := json.NewDecoder(file).Decode(&ctx); err != nil {
		return nil, fmt.Errorf("failed to decode security context from '%s': %w", path, err)
	}
	log.Printf("Loaded security context from %s. Fingerprint: %s", path, ctx.CertificateHash)
	return &ctx, nil
}
