package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/bg-dao/axon-codex-sceneops/internal/mcp"
)

func main() {
	root := flag.String("project", "", "absolute SceneOps project root")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "--project is required")
		os.Exit(2)
	}
	if err := mcp.Run(context.Background(), *root, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
