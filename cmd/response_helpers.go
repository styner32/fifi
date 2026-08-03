package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/parse"
)

func printAPIResult(name string, resp *auth.RESTResponse, outputKey string) {
	if resp == nil {
		fmt.Printf("%s: response is nil\n", name)
		return
	}

	fmt.Printf("\n[%s]\n", name)
	fmt.Printf("rt_cd=%v msg_cd=%v msg1=%v\n", resp.Body["rt_cd"], resp.Body["msg_cd"], resp.Body["msg1"])
	if resp.ParseError != "" {
		fmt.Printf("status_code=%d content_type=%s body_bytes=%d\n", resp.StatusCode, resp.Headers.Get("Content-Type"), len(resp.RawBody))
		if resp.RequestMethod != "" || resp.RequestURL != "" {
			fmt.Printf("request=%s %s\n", resp.RequestMethod, resp.RequestURL)
		}
		fmt.Printf("parse_error=%s\n", resp.ParseError)
		if len(resp.RawBody) > 0 {
			fmt.Printf("raw_body=%s\n", rawPreview(resp.RawBody))
		}
		if resp.ParseError == "empty response body" {
			fmt.Printf("note=server returned no JSON body; this endpoint may require additional parameters or may not be exposed in the current environment\n")
		}
		return
	}
	if outputKey == "" {
		if len(resp.RawBody) > 0 && len(resp.Body) == 0 {
			fmt.Printf("raw_body=%s\n", rawPreview(resp.RawBody))
		}
		return
	}
	if value, ok := resp.Body[outputKey]; ok {
		fmt.Printf("output(%s)=%s\n", outputKey, preview(value))
		return
	}
	if len(resp.RawBody) > 0 {
		fmt.Printf("raw_body=%s\n", rawPreview(resp.RawBody))
	}
}

func mustAPIResult(name string, resp *auth.RESTResponse, err error, outputKey string) {
	if err != nil {
		log.Fatalf("%s error: %v", name, err)
	}
	if resp == nil {
		log.Fatalf("%s error: response is nil", name)
	}
	if !resp.IsOK() {
		log.Fatalf("%s error: msg_cd=%s msg1=%s", name, resp.MessageCode(), resp.Message())
	}

	printAPIResult(name, resp, outputKey)
}

func optionalAPIResult(name string, resp *auth.RESTResponse, err error, outputKey string) bool {
	if err != nil {
		if isEmptyBodyResponse(resp, err) {
			log.Printf("%s returned an empty response body", name)
			if resp != nil {
				printAPIResult(name, resp, outputKey)
			}
			return false
		}
		log.Printf("%s error: %v", name, err)
		if resp != nil {
			printAPIResult(name, resp, outputKey)
		}
		return false
	}
	if resp == nil {
		log.Printf("%s error: response is nil", name)
		return false
	}
	if !resp.IsOK() {
		log.Printf("%s error: msg_cd=%s msg1=%s", name, resp.MessageCode(), resp.Message())
		return false
	}

	printAPIResult(name, resp, outputKey)
	return true
}

func isEmptyBodyResponse(resp *auth.RESTResponse, err error) bool {
	if resp == nil {
		return false
	}

	if resp.ParseError == "empty response body" {
		return true
	}

	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "empty response body")
}

func preview(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		return fmt.Sprintf("%v", typed)
	case []any:
		if len(typed) == 0 {
			return "[]"
		}
		return fmt.Sprintf("len=%d first=%v", len(typed), typed[0])
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func rawPreview(raw []byte) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "<empty>"
	}
	const limit = 400
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "...(truncated)"
}

func firstRow(resp *auth.RESTResponse, outputKey string) map[string]any {
	return resp.FirstRow(outputKey)
}

func firstRowAny(resp *auth.RESTResponse, outputKeys ...string) map[string]any {
	return resp.FirstRow(outputKeys...)
}

func rowsFromResponse(resp *auth.RESTResponse, outputKey string) []map[string]any {
	return resp.Rows(outputKey)
}

func findRowByValue(rows []map[string]any, key string, target string) map[string]any {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	for _, row := range rows {
		if fieldString(row, key) == target {
			return row
		}
	}
	return nil
}

func firstNonEmpty(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value := fieldString(row, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func fieldString(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func fieldFloat(row map[string]any, key string) (float64, bool) {
	return parse.Num(row, key)
}

func computeBasis(row map[string]any) (float64, bool) {
	if basis, ok := fieldFloat(row, "basis"); ok {
		return basis, true
	}

	futuresPrice, ok := fieldFloat(row, "futs_prpr")
	if !ok {
		return 0, false
	}
	spotIndex, ok := fieldFloat(row, "bstp_nmix_prpr")
	if !ok {
		return 0, false
	}

	return futuresPrice - spotIndex, true
}
