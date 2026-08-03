package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"
)

func main() {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
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
