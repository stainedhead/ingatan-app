package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadClientCA_ValidFile(t *testing.T) {
	caFile := writeTempCA(t)
	pool, err := LoadClientCA(caFile)
	require.NoError(t, err)
	assert.NotNil(t, pool)
}

func TestLoadClientCA_NotFound(t *testing.T) {
	_, err := LoadClientCA("/nonexistent/ca.pem")
	assert.Error(t, err)
}

func TestLoadClientCA_EmptyFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "empty-ca*.pem")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	_, err = LoadClientCA(f.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid CA certificates")
}

func TestApplyClientAuth_Required(t *testing.T) {
	pool := x509.NewCertPool()
	cfg := &tls.Config{}
	ApplyClientAuth(cfg, ClientAuthRequired, pool)
	assert.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	assert.Equal(t, pool, cfg.ClientCAs)
}

func TestApplyClientAuth_Optional(t *testing.T) {
	pool := x509.NewCertPool()
	cfg := &tls.Config{}
	ApplyClientAuth(cfg, ClientAuthOptional, pool)
	assert.Equal(t, tls.VerifyClientCertIfGiven, cfg.ClientAuth)
	assert.Equal(t, pool, cfg.ClientCAs)
}

func TestApplyClientAuth_None(t *testing.T) {
	cfg := &tls.Config{}
	ApplyClientAuth(cfg, ClientAuthNone, nil)
	assert.Equal(t, tls.NoClientCert, cfg.ClientAuth)
}

func TestClientCertCN_NoTLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Equal(t, "", ClientCertCN(req))
}

func TestClientCertCN_WithVerifiedCert(t *testing.T) {
	cert, _, err := generateSelfSignedCert("test-agent")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{cert}},
	}
	assert.Equal(t, "test-agent", ClientCertCN(req))
}

func TestClientCertCN_EmptyVerifiedChains(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{},
	}
	assert.Equal(t, "", ClientCertCN(req))
}

// writeTempCA generates a self-signed CA cert, writes it to a temp file, and returns the path.
func writeTempCA(t *testing.T) string {
	t.Helper()
	cert, pemBytes, err := generateSelfSignedCert("test-ca")
	require.NoError(t, err)
	_ = cert

	dir := t.TempDir()
	path := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))
	return path
}

// generateSelfSignedCert creates a self-signed certificate with the given CN.
// Returns the parsed certificate and its PEM-encoded bytes.
func generateSelfSignedCert(cn string) (*x509.Certificate, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return cert, pemBytes, nil
}
