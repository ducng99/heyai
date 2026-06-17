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
	auto, promptArgs := parseRuntimeFlags(args)
	if len(promptArgs) == 0 {
		return errors.New("missing prompt")
	}

	client := NewOpenAIClient(cfg)
	chat := Chat{Client: client, Config: cfg, Auto: auto, Out: os.Stdout, Err: os.Stderr}
	if auto {
		chat.AutoClient = NewAutoCheckClient(cfg)
	}
	return chat.Run(context.Background(), strings.Join(promptArgs, " "))
}

func parseRuntimeFlags(args []string) (bool, []string) {
	auto := false
	promptArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--auto" || arg == "-a" {
			auto = true
			continue
		}
		promptArgs = append(promptArgs, arg)
	}
	return auto, promptArgs
}

func printUsage() {
	fmt.Println(`Usage:
	  heyai [--auto|-a] "prompt here"
	  heyai --init
	  heyai --config-path

--auto, -a asks an AI safety checker to approve commands that would otherwise need confirmation.

Configuration is read from $XDG_CONFIG_HOME/heyai/config.json or ~/.config/heyai/config.json.`)
}
