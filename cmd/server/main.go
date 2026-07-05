package main

import (
	"fmt"
	"os"
)

func main() {
	if _, err := fmt.Fprintln(os.Stderr, "fileserver-mcp bootstrap placeholder"); err != nil {
		os.Exit(1)
	}
}
