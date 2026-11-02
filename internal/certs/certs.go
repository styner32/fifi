package certs

import (
	_ "embed"
)

//go:embed kofia_cert.pem
var KofiaCert []byte
