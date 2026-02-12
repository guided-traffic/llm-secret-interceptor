package main

import (
	"fmt"
	"os"

	"github.com/hfi/llm-secret-interceptor/internal/config"
)

var (
	// Version information - set at build time
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("LLM Secret Interceptor %s\n", Version)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("LLM Secret Interceptor %s starting...\n", Version)
	fmt.Printf("Listening on %s\n", cfg.Proxy.Listen)

	// TODO: Initialize and start proxy server
	// TODO: Initialize TLS interception layer
	// TODO: Initialize protocol handlers
	// TODO: Initialize secret interceptor manager
	// TODO: Initialize mapping store
	// TODO: Initialize metrics endpoint

	// Placeholder - will be replaced with actual server
	fmt.Println("Proxy server not yet implemented")
}
