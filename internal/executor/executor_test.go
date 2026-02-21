package executor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/platform"
)

func setupTestEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	platform.SetBaseDir(dir)
	t.Cleanup(func() { platform.SetBaseDir("") })

	// Create prompts directory and a test prompt
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("Do something."), 0644)

	// Create summary directory
	os.MkdirAll(filepath.Join(dir, "summary"), 0755)

	return dir
}

func TestBuildCommand_BasicArgs(t *testing.T) {
	setupTestEnv(t)

	job := &config.JobConfig{
		Name:       "test",
		PromptFile: "test.md",
		WorkingDir: t.TempDir(),
	}

	result, err := BuildCommand(context.Background(), job)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	args := result.Cmd.Args
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-p") {
		t.Error("expected -p flag")
	}
	if !strings.Contains(argsStr, "--permission-mode bypassPermissions") {
		t.Error("expected --permission-mode bypassPermissions")
	}
	if !strings.Contains(argsStr, "--output-format json") {
		t.Error("expected --output-format json")
	}
}

func TestBuildCommand_WithModel(t *testing.T) {
	setupTestEnv(t)

	job := &config.JobConfig{
		Name:       "test",
		PromptFile: "test.md",
		WorkingDir: t.TempDir(),
		Model:      "opus",
	}

	result, err := BuildCommand(context.Background(), job)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	argsStr := strings.Join(result.Cmd.Args, " ")
	if !strings.Contains(argsStr, "--model opus") {
		t.Errorf("expected --model opus in args: %s", argsStr)
	}
}

func TestBuildCommand_WithEffort(t *testing.T) {
	setupTestEnv(t)

	job := &config.JobConfig{
		Name:       "test",
		PromptFile: "test.md",
		WorkingDir: t.TempDir(),
		Effort:     "max",
	}

	result, err := BuildCommand(context.Background(), job)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	argsStr := strings.Join(result.Cmd.Args, " ")
	if !strings.Contains(argsStr, "--effort max") {
		t.Errorf("expected --effort max in args: %s", argsStr)
	}
}

func TestBuildCommand_WithDisallowedTools(t *testing.T) {
	setupTestEnv(t)

	job := &config.JobConfig{
		Name:            "test",
		PromptFile:      "test.md",
		WorkingDir:      t.TempDir(),
		DisallowedTools: []string{"Bash(git:*)", "Edit"},
	}

	result, err := BuildCommand(context.Background(), job)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	argsStr := strings.Join(result.Cmd.Args, " ")
	if !strings.Contains(argsStr, "--disallowedTools") {
		t.Errorf("expected --disallowedTools in args: %s", argsStr)
	}
}

func TestBuildCommand_NoSessionPersistence(t *testing.T) {
	setupTestEnv(t)

	job := &config.JobConfig{
		Name:             "test",
		PromptFile:       "test.md",
		WorkingDir:       t.TempDir(),
		NoSessionPersist: true,
	}

	result, err := BuildCommand(context.Background(), job)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	argsStr := strings.Join(result.Cmd.Args, " ")
	if !strings.Contains(argsStr, "--no-session-persistence") {
		t.Errorf("expected --no-session-persistence in args: %s", argsStr)
	}
}

func TestBuildCommand_SummaryEnabled(t *testing.T) {
	setupTestEnv(t)

	job := &config.JobConfig{
		Name:           "test",
		PromptFile:     "test.md",
		WorkingDir:     t.TempDir(),
		SummaryEnabled: true,
	}

	result, err := BuildCommand(context.Background(), job)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	if result.SummaryPath == "" {
		t.Error("expected SummaryPath to be set when SummaryEnabled is true")
	}
}

func TestBuildCommand_PromptFileNotFound(t *testing.T) {
	setupTestEnv(t)

	job := &config.JobConfig{
		Name:       "test",
		PromptFile: "nonexistent.md",
		WorkingDir: t.TempDir(),
	}

	_, err := BuildCommand(context.Background(), job)
	if err == nil {
		t.Error("expected error for missing prompt file")
	}
}

func TestBuildCommand_PromptContent(t *testing.T) {
	setupTestEnv(t)

	promptContent := "Run the tests and report results."
	os.WriteFile(filepath.Join(platform.PromptsDir(), "content-test.md"), []byte(promptContent), 0644)

	job := &config.JobConfig{
		Name:       "test",
		PromptFile: "content-test.md",
		WorkingDir: t.TempDir(),
	}

	result, err := BuildCommand(context.Background(), job)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	// The command's stdin should contain the preamble + prompt
	if result.Cmd.Stdin == nil {
		t.Fatal("expected stdin to be set (prompt piped via stdin)")
	}
}

func TestParseUsage_SingleJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout.json")

	output := claudeOutput{
		Result:       "task completed",
		TotalCostUSD: 0.05,
	}
	output.Usage.InputTokens = 1000
	output.Usage.OutputTokens = 500

	data, _ := json.Marshal(output)
	os.WriteFile(path, data, 0644)

	result := &Result{}
	parseUsage(path, result)

	if result.Output != "task completed" {
		t.Errorf("Output = %q, want %q", result.Output, "task completed")
	}
	if result.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", result.CostUSD)
	}
	if result.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", result.InputTokens)
	}
}

func TestParseUsage_NDJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout.json")

	// Multiple lines, last one has the result
	lines := []string{
		`{"type":"progress","result":"","total_cost_usd":0.01}`,
		`{"type":"result","result":"final answer","total_cost_usd":0.05,"usage":{"input_tokens":2000,"output_tokens":800}}`,
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)

	result := &Result{}
	parseUsage(path, result)

	if result.Output != "final answer" {
		t.Errorf("Output = %q, want %q", result.Output, "final answer")
	}
	if result.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", result.CostUSD)
	}
}

func TestParseUsage_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout.json")
	os.WriteFile(path, []byte(""), 0644)

	result := &Result{}
	parseUsage(path, result) // should not panic

	if result.Output != "" {
		t.Errorf("Output = %q, want empty", result.Output)
	}
}

func TestParseUsage_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout.json")
	os.WriteFile(path, []byte("this is not json"), 0644)

	result := &Result{}
	parseUsage(path, result) // should not panic

	if result.Output != "" {
		t.Errorf("Output = %q, want empty", result.Output)
	}
}
