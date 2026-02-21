// File anthropic.go implements the Provider interface for Anthropic's Claude Code CLI.
// It detects the claude binary on PATH via exec.LookPath, verifies authentication
// by running "claude --version", and reports the installed CLI version string.
package provider

import (
	"os/exec"
	"strings"
)

// Anthropic implements the Provider interface for Claude Code.
type Anthropic struct{}

func (a *Anthropic) ID() string   { return "anthropic" }
func (a *Anthropic) Name() string { return "Anthropic (Claude Code)" }

func (a *Anthropic) Detect() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (a *Anthropic) CheckAuth() error {
	cmd := exec.Command("claude", "--version")
	return cmd.Run()
}

func (a *Anthropic) Version() string {
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
