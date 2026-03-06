// Package middleware: mtls provides helpers for mTLS client certificate handling.
package middleware

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
)

// ClientAuthMode represents the mTLS client authentication requirement.
type ClientAuthMode string

const (
	// ClientAuthNone disables client certificate verification.
	ClientAuthNone ClientAuthMode = "none"
	// ClientAuthOptional requests a client certificate but does not require it.
	ClientAuthOptional ClientAuthMode = "optional"
	// ClientAuthRequired requires a valid client certificate.
	ClientAuthRequired ClientAuthMode = "required"
)

// LoadClientCA reads a PEM-encoded CA certificate file and returns an x509.CertPool.
// Returns an error if the file cannot be read or contains no valid certificates.
func LoadClientCA(caFile string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, &invalidCAError{file: caFile}
	}
	return pool, nil
}

// ApplyClientAuth configures mTLS on a tls.Config based on the ClientAuthMode.
// clientCAs may be nil when mode is ClientAuthNone.
func ApplyClientAuth(cfg *tls.Config, mode ClientAuthMode, clientCAs *x509.CertPool) {
	switch mode {
	case ClientAuthRequired:
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.ClientCAs = clientCAs
	case ClientAuthOptional:
		cfg.ClientAuth = tls.VerifyClientCertIfGiven
		cfg.ClientCAs = clientCAs
	default:
		// ClientAuthNone: leave ClientAuth at default (tls.NoClientCert)
	}
}

// ClientCertCN extracts the Common Name from the first verified peer certificate
// on the TLS connection of the request. Returns "" if no TLS or no peer cert.
func ClientCertCN(r *http.Request) string {
	if r.TLS == nil {
		return ""
	}
	if len(r.TLS.VerifiedChains) == 0 {
		return ""
	}
	if len(r.TLS.VerifiedChains[0]) == 0 {
		return ""
	}
	return r.TLS.VerifiedChains[0][0].Subject.CommonName
}

// invalidCAError is returned when a CA file contains no valid PEM certificates.
type invalidCAError struct {
	file string
}

func (e *invalidCAError) Error() string {
	return "no valid CA certificates found in file: " + e.file
}
