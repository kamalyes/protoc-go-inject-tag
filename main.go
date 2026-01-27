package main

import (
	"fmt"
	"os"

	"github.com/kamalyes/protoc-go-inject-tag/bootstrap"
)

func main() {
	if err := bootstrap.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
