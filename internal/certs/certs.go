package certs

import (
	"crypto/x509"
	_ "embed"
	"fmt"
)

//go:embed kofia.pem
var KofiaCertPEM []byte

// GetKofiaRootCAs returns an *x509.CertPool containing the embedded GlobalSign certificates
// necessary to verify the freesis.kofia.or.kr server certificate.
func GetKofiaRootCAs() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if ok := pool.AppendCertsFromPEM(KofiaCertPEM); !ok {
		return nil, fmt.Errorf("failed to append embedded Kofia/GlobalSign certificates to pool")
	}
	return pool, nil
}
