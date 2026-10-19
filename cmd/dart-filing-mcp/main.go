// Compatibility entry point for the unified server. Prefer cmd/fifi-mcp.
package main

import (
	"log"
	"os"

	"github.com/fifi/internal/fifimcp"
)

func main() {
	if err := fifimcp.RunStdio(); err != nil {
		log.Printf("fifi MCP stopped: %v", err)
		os.Exit(1)
	}
}
