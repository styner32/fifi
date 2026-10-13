package kofia

import (
	"crypto/x509"
	_ "embed"
	"fmt"
)

//go:embed kofia_cert.pem
var kofiaCertPEM []byte

// GetKofiaCertPool returns an x509.CertPool containing the KOFIA FreeSIS certificates
// and system certificates (if available).
func GetKofiaCertPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	if ok := pool.AppendCertsFromPEM(kofiaCertPEM); !ok {
		return nil, fmt.Errorf("failed to parse KOFIA FreeSIS embedded certificates")
	}

	return pool, nil
}
