package certs

import (
	_ "embed"
)

//go:embed freesis.pem
var FreeSISCerts []byte
