package tool

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type ToolOptions struct {
	In  io.Reader
	Err io.Writer
}

func isSensitivePath(path string) bool {
	name := filepath.Base(path)
	if name == ".env" || strings.HasPrefix(name, ".env.") {
		return true
	}
	if name == ".netrc" || name == ".npmrc" || name == ".git-credentials" {
		return true
	}
	if name == "credentials.json" || name == "service-account.json" || strings.HasPrefix(name, "service-account-") {
		return true
	}
	if name == "secrets.yml" || name == "secrets.yaml" || name == "secrets.json" {
		return true
	}
	if strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") {
		return true
	}
	if name == "config.json" && strings.Contains(path, ".aws") {
		return true
	}
	if name == "config" && strings.Contains(path, ".kube") {
		return true
	}
	if name == "config.json" && strings.Contains(path, ".docker") {
		return true
	}
	if name == "credentials.db" && strings.Contains(path, ".gcloud") {
		return true
	}
	return false
}

func confirmToolAction(options ToolOptions, toolName, reason, target string) bool {
	if options.Err == nil {
		options.Err = io.Discard
	}
	if options.In == nil {
		options.In = strings.NewReader("")
	}
	fmt.Fprintf(options.Err, "⚠️  %s: %s\n%s? [y/N] ", reason, target, toolName)
	answer, _ := bufio.NewReader(options.In).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
