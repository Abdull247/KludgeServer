package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "run the api with: go run ./cmd/server")
	os.Exit(1)
}
