package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}

	switch args[0] {
	case "--config-path":
		path, err := configPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	case "--init":
		return initConfig()
	}

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return errors.New("missing api_key in config")
	}

	client := NewOpenAIClient(cfg)
	chat := Chat{Client: client, Config: cfg, Out: os.Stdout, Err: os.Stderr}
	return chat.Run(context.Background(), strings.Join(args, " "))
}

func printUsage() {
	fmt.Println(`Usage:
  heyai "prompt here"
  heyai --init
  heyai --config-path

Configuration is read from $XDG_CONFIG_HOME/heyai/config.json or ~/.config/heyai/config.json.`)
}
