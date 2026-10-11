package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"
)

var kofiaCertPEM = []byte(`
-----BEGIN CERTIFICATE-----
MIIGbTCCBVWgAwIBAgIMdASdnGHzthmyNtWcMA0GCSqGSIb3DQEBCwUAMFAxCzAJ
BgNVBAYTAkJFMRkwFwYDVQQKExBHbG9iYWxTaWduIG52LXNhMSYwJAYDVQQDEx1H
bG9iYWxTaWduIFJTQSBPViBTU0wgQ0EgMjAxODAeFw0yNjAzMDQwOTEzMjdaFw0y
NzA0MDUwOTEzMjZaMIGAMQswCQYDVQQGEwJLUjEOMAwGA1UECBMFU2VvdWwxGDAW
BgNVBAcTD1llb25nZGV1bmdwby1ndTEvMC0GA1UEChMmS29yZWEgRmluYW5jaWFs
IEludmVzdG1lbnQgQXNzb2NpYXRpb24xFjAUBgNVBAMMDSoua29maWEub3Iua3Iw
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCq9tsT35DWDa0xVwQ1+lV0
LncpMW/5srjDoBVWObS/HaCGtXpjuZEbVu2s3nWtQx8mTpJdgvjmuT0VySoYcS/R
DG9TqXgiHTEcO4pK/i1fuDsHAXyA44DVif5F+975rWfvfStujrlCx4KBVyWdPzZj
qu5umxWM8v74C+o3fCo+3VV7sqjwNlwW1g/fTlfkDVFRSZ3LM82y6CzmUB1liLxZ
4d3qtWzj96h5BNnSjcu5i9FcMmk+unppXC7BFUZ4kVdU/yX3Rd1GlQBD2sPAh4Ge
MA6e03z/PtzbeaywpE5sBFIR4xfYhi3AplDowx2lBBtuAeMwG7jSyl7WVbFjaoBd
AgMBAAGjggMUMIIDEDAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/BAIwADCBjgYI
KwYBBQUHAQEEgYEwfzBEBggrBgEFBQcwAoY4aHR0cDovL3NlY3VyZS5nbG9iYWxz
aWduLmNvbS9jYWNlcnQvZ3Nyc2FvdnNzbGNhMjAxOC5jcnQwNwYIKwYBBQUHMAGG
K2h0dHA6Ly9vY3NwLmdsb2JhbHNpZ24uY29tL2dzcnNhb3Zzc2xjYTIwMTgwVgYD
VR0gBE8wTTBBBgkrBgEEAaAyARQwNDAyBggrBgEFBQcCARYmaHR0cHM6Ly93d3cu
Z2xvYmFsc2lnbi5jb20vcmVwb3NpdG9yeS8wCAYGZ4EMAQICMCUGA1UdEQQeMByC
DSoua29maWEub3Iua3KCC2tvZmlhLm9yLmtyMB0GA1UdJQQWMBQGCCsGAQUFBwMB
BggrBgEFBQcDAjAfBgNVHSMEGDAWgBT473/yzXhnqN5vjySNiPGHAwKz6zAdBgNV
HQ4EFgQUSE+fINss6eS+4QhVGO79gfKNbNkwggF/BgorBgEEAdZ5AgQCBIIBbwSC
AWsBaQB2AByfaCzp+vBFaVD4G5aKh93bMhDYTObIsuOCUkrEz1mfAAABnLgfsVIA
AAQDAEcwRQIgOrOgvqAdwLbUW21xcn27nnUU4EBjAWbC3UDFSHuigP8CIQCo5nLK
wTr7ODZ9T90U9rRfXhj3XfkjVSUwvl/MD469TgB3AExj3JjlnB2riPYeij3ero+r
RKM3e1+blMP7oZz8wb4mAAABnLgfsS0AAAQDAEgwRgIhAMqoFd96OjAdwhqvvU5e
hgDHarGjC4e9iv3AGFiSg8CPAiEAtyborHpjsySQ0K/2lAFucZevDzKN8Y42aoSJ
aK1smsYAdgBgTJqven93XwHUBvySDciZ6wscffjJUhv6+hd3O5eLyQAAAZy4H7Ig
AAAEAwBHMEUCIQDWBr+Uoi3MkKtEY44sRFy2eslEZ0rE0GaNn0qeElJe6wIgMpUg
rl8Hq/P7AY2O7nEzRZM6P7LWmfKw90NCiUrokoUwDQYJKoZIhvcNAQELBQADggEB
AFt1Zj8LoIRTDY9OjVVv3rDXmRL5ybZTvaBL2JdlU8zSWK11fHRFxErYM+CRBkrf
ZN5ALwkG9Uk9CWQXbBGBix672CH6FWIZsy8eeQZap2WFeWjPXwkO/9oYxyD+alG+
jVR03Jx+KkF04pbZnTRNWZMjitWf5sr+jDo7Jo7skZL6UwGqCMTLFFg7Mj4LD8bx
buGrQcTjQcnoQ/xHqNmQbeKmssh9oPpoFaJ4FdG/ZpjCO+MAk0yhkw7n62j38l0O
nK8wV6nUYdtjkR5A7ylPkZ1vO8yWUmhh/ZlhaYRgZmac29DGtdLfOM5xlqGAAEn0
2VE6DhAbIRDeoGSaHb3l/UI=
-----END CERTIFICATE-----
`)

func main() {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(kofiaCertPEM)

	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
	client := &http.Client{Timeout: 15 * time.Second, Jar: jar, Transport: transport}
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"

	// Step 1: establish session
	fmt.Fprintln(os.Stderr, "[1] Establishing session...")
	req, _ := http.NewRequest("GET", "https://freesis.kofia.or.kr/index.jsp", nil)
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session error: %v\n", err)
		os.Exit(1)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fmt.Fprintf(os.Stderr, "[1] Session status: %d, cookies: %d\n", resp.StatusCode, len(client.Jar.Cookies(req.URL)))

	// Step 2: fetch data
	fmt.Fprintln(os.Stderr, "[2] Fetching data...")
	payload := map[string]any{
		"dmSearch": map[string]any{
			"tmpV40": "100",
			"tmpV41": "1",
			"tmpV1":  "D",
			"tmpV45": "20260601",
			"tmpV46": "20260604",
			"OBJ_NM": "STATSCU0100000060BO",
		},
	}
	body, _ := json.Marshal(payload)
	req2, _ := http.NewRequest("POST", "https://freesis.kofia.or.kr/meta/getMetaDataList.do", bytes.NewReader(body))
	req2.Header.Set("User-Agent", ua)
	req2.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req2.Header.Set("Accept", "*/*")
	req2.Header.Set("Origin", "https://freesis.kofia.or.kr")
	req2.Header.Set("Referer", "https://freesis.kofia.or.kr/stat/FreeSIS.do?parentDivId=MSIS10000000000000&serviceId=STATSCU0100000060")
	req2.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
		os.Exit(1)
	}
	defer resp2.Body.Close()

	raw, _ := io.ReadAll(resp2.Body)
	fmt.Fprintf(os.Stderr, "[2] Response status: %d, length: %d bytes\n", resp2.StatusCode, len(raw))

	// Pretty print
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, raw, "", "  "); err != nil {
		// Not JSON, print raw
		fmt.Println(string(raw))
	} else {
		// Print first 3000 chars
		out := prettyJSON.String()
		if len(out) > 3000 {
			out = out[:3000] + "\n... (truncated)"
		}
		fmt.Println(out)
	}
}
