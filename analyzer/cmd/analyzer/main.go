// Command analyzer scans a repository path and emits pipeline-stage
// documents (plan tenara-agent-paas#23, RB-10).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"tenara/analyzer/internal/facts"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "scan" {
		fmt.Fprintln(os.Stderr, "usage: analyzer scan --path DIR [--stage STAGE]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	path := fs.String("path", ".", "repository path to analyze")
	stage := fs.String("stage", "facts", "pipeline stage (facts)")
	if parseErr := fs.Parse(os.Args[2:]); parseErr != nil {
		os.Exit(2)
	}
	switch *stage {
	case "facts":
		out, buildErr := facts.Build(*path)
		if buildErr != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", buildErr)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(out); encodeErr != nil {
			fmt.Fprintf(os.Stderr, "encode failed: %v\n", encodeErr)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown stage %q\n", *stage)
		os.Exit(2)
	}
}
