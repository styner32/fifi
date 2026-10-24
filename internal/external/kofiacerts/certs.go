package kofiacerts

import (
	"crypto/x509"
	_ "embed"
	"log"
	"sync"
)

//go:embed freesis.pem
var FreesisCerts []byte

var (
	rootCAsOnce sync.Once
	cachedPool  *x509.CertPool
)

// GetRootCAs creates an x509.CertPool containing system CAs
// and appends the embedded FreeSIS certificates. It caches the result.
func GetRootCAs() *x509.CertPool {
	rootCAsOnce.Do(func() {
		pool, err := x509.SystemCertPool()
		if err != nil {
			log.Printf("kofiacerts: failed to load system cert pool, creating empty pool: %v", err)
			pool = x509.NewCertPool()
		}

		if ok := pool.AppendCertsFromPEM(FreesisCerts); !ok {
			log.Printf("kofiacerts: failed to append embedded FreeSIS certificates to pool")
		}
		cachedPool = pool
	})
	return cachedPool
}
