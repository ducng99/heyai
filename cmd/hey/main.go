package main

import (
	"fmt"
	"os"

	"github.com/ducng99/heyai"
)

func main() {
	if err := heyai.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
