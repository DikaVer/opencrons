package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestJobConfig_Validate_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		job     JobConfig
		wantErr string
	}{
		{"missing name", JobConfig{}, "job name is required"},
		{"missing schedule", JobConfig{Name: "test"}, "schedule is required"},
		{"missing prompt_file", JobConfig{Name: "test", Schedule: "* * * * *"}, "prompt_file is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, got)
			}
		})
	}
}

func TestJobConfig_Validate_WorkingDir(t *testing.T) {
	existingDir := t.TempDir()
	nonexistentDir := filepath.Join(t.TempDir(), "does-not-exist")

	// Create a file (not a directory) to test the is-not-a-dir branch
	fileNotDir := filepath.Join(t.TempDir(), "a-file.txt")
	if err := os.WriteFile(fileNotDir, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		workingDir string
		wantErr    bool
	}{
		{"empty (project folder)", "", false},
		{"valid existing dir", existingDir, false},
		{"nonexistent dir", nonexistentDir, true},
		{"path is a file not a dir", fileNotDir, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := JobConfig{
				Name:       "test",
				Schedule:   "* * * * *",
				PromptFile: "test.md",
				WorkingDir: tt.workingDir,
			}
			err := job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobConfig_Validate_NameFormat(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		jobName string
		wantErr bool
	}{
		{"valid simple", "my-job", false},
		{"valid underscore", "my_job_2", false},
		{"invalid spaces", "my job", true},
		{"invalid special chars", "my@job", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := JobConfig{
				Name:       tt.jobName,
				Schedule:   "* * * * *",
				PromptFile: "test.md",
				WorkingDir: dir,
			}
			err := job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobConfig_Validate_PromptFileSecurity(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name       string
		promptFile string
		wantErr    bool
	}{
		{"valid relative", "test.md", false},
		{"path traversal", "../etc/passwd", true},
		{"double dot in name", "te..st.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := JobConfig{
				Name:       "test",
				Schedule:   "* * * * *",
				PromptFile: tt.promptFile,
				WorkingDir: dir,
			}
			err := job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobConfig_Validate_CronSchedule(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		schedule string
		wantErr  bool
	}{
		{"valid standard", "0 2 * * *", false},
		{"valid every minute", "* * * * *", false},
		{"invalid", "not a cron", true},
		{"invalid too few fields", "* *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := JobConfig{
				Name:       "test",
				Schedule:   tt.schedule,
				PromptFile: "test.md",
				WorkingDir: dir,
			}
			err := job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobConfig_Validate_Model(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{"empty (default)", "", false},
		{"valid sonnet", "sonnet", false},
		{"valid opus", "opus", false},
		{"valid haiku", "haiku", false},
		{"invalid", "gpt-4", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := JobConfig{
				Name:       "test",
				Schedule:   "* * * * *",
				PromptFile: "test.md",
				WorkingDir: dir,
				Model:      tt.model,
			}
			err := job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobConfig_Validate_Effort(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		effort  string
		wantErr bool
	}{
		{"empty (default)", "", false},
		{"valid low", "low", false},
		{"valid high", "high", false},
		{"valid max", "max", false},
		{"invalid", "extreme", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := JobConfig{
				Name:       "test",
				Schedule:   "* * * * *",
				PromptFile: "test.md",
				WorkingDir: dir,
				Effort:     tt.effort,
			}
			err := job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobConfig_Validate_Timeout(t *testing.T) {
	dir := t.TempDir()

	job := JobConfig{
		Name:       "test",
		Schedule:   "* * * * *",
		PromptFile: "test.md",
		WorkingDir: dir,
		Timeout:    -1,
	}
	if err := job.Validate(); err == nil {
		t.Error("expected error for negative timeout")
	}

	job.Timeout = 300
	if err := job.Validate(); err != nil {
		t.Errorf("unexpected error for positive timeout: %v", err)
	}
}

func TestLoadJob(t *testing.T) {
	dir := t.TempDir()
	content := `id: abc123
name: test-job
schedule: "0 2 * * *"
working_dir: .
prompt_file: test.md
model: sonnet
enabled: true
`
	path := filepath.Join(dir, "test-job.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	job, err := LoadJob(path)
	if err != nil {
		t.Fatalf("LoadJob: %v", err)
	}

	if job.Name != "test-job" {
		t.Errorf("Name = %q, want %q", job.Name, "test-job")
	}
	if job.Schedule != "0 2 * * *" {
		t.Errorf("Schedule = %q, want %q", job.Schedule, "0 2 * * *")
	}
	if job.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", job.Model, "sonnet")
	}
	if !job.Enabled {
		t.Error("Enabled = false, want true")
	}
}

func TestSaveJob_LoadJob_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	job := &JobConfig{
		ID:               "abc12345",
		Name:             "roundtrip",
		Schedule:         "*/5 * * * *",
		WorkingDir:       ".",
		PromptFile:       "roundtrip.md",
		Model:            "opus",
		Effort:           "max",
		Timeout:          600,
		SummaryEnabled:   true,
		NoSessionPersist: true,
		Enabled:          true,
	}

	if err := SaveJob(dir, job); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	loaded, err := LoadJob(filepath.Join(dir, "roundtrip.yml"))
	if err != nil {
		t.Fatalf("LoadJob: %v", err)
	}

	if loaded.Name != job.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, job.Name)
	}
	if loaded.Model != job.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, job.Model)
	}
	if loaded.Effort != job.Effort {
		t.Errorf("Effort = %q, want %q", loaded.Effort, job.Effort)
	}
	if loaded.Timeout != job.Timeout {
		t.Errorf("Timeout = %d, want %d", loaded.Timeout, job.Timeout)
	}
	if loaded.SummaryEnabled != job.SummaryEnabled {
		t.Errorf("SummaryEnabled = %v, want %v", loaded.SummaryEnabled, job.SummaryEnabled)
	}
}

func TestLoadAllJobs(t *testing.T) {
	dir := t.TempDir()

	// Write two valid configs
	for _, name := range []string{"job-a", "job-b"} {
		content := "id: x\nname: " + name + "\nschedule: \"* * * * *\"\nworking_dir: .\nprompt_file: " + name + ".md\nenabled: true\n"
		if err := os.WriteFile(filepath.Join(dir, name+".yml"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Write one malformed config
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("not: [valid: yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a non-YAML file (should be skipped)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore"), 0644); err != nil {
		t.Fatal(err)
	}

	jobs, err := LoadAllJobs(dir)
	if err != nil {
		t.Fatalf("LoadAllJobs: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("got %d jobs, want 2", len(jobs))
	}
}

func TestDeleteJob(t *testing.T) {
	schedDir := t.TempDir()
	promptDir := t.TempDir()

	// Create a job
	job := &JobConfig{Name: "delme", Schedule: "* * * * *", WorkingDir: ".", PromptFile: "delme.md", Enabled: true}
	if err := SaveJob(schedDir, job); err != nil {
		t.Fatal(err)
	}
	if err := SavePromptFile(promptDir, "delme.md", "test prompt"); err != nil {
		t.Fatal(err)
	}

	// Delete it
	if err := DeleteJob(schedDir, promptDir, "delme", true); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// Verify config is gone
	if JobNameExists(schedDir, "delme") {
		t.Error("job config still exists after delete")
	}

	// Verify prompt is gone
	if _, err := os.Stat(filepath.Join(promptDir, "delme.md")); err == nil {
		t.Error("prompt file still exists after delete")
	}
}

func TestJobNameExists(t *testing.T) {
	dir := t.TempDir()

	if JobNameExists(dir, "nonexistent") {
		t.Error("JobNameExists returned true for nonexistent job")
	}

	if err := os.WriteFile(filepath.Join(dir, "exists.yml"), []byte("name: exists"), 0644); err != nil {
		t.Fatal(err)
	}
	if !JobNameExists(dir, "exists") {
		t.Error("JobNameExists returned false for existing job")
	}
}

func TestFindJobByName(t *testing.T) {
	dir := t.TempDir()

	content := "id: x\nname: findme\nschedule: \"* * * * *\"\nworking_dir: .\nprompt_file: findme.md\n"
	if err := os.WriteFile(filepath.Join(dir, "findme.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	job, err := FindJobByName(dir, "findme")
	if err != nil {
		t.Fatalf("FindJobByName: %v", err)
	}
	if job.Name != "findme" {
		t.Errorf("Name = %q, want %q", job.Name, "findme")
	}

	_, err = FindJobByName(dir, "missing")
	if err == nil {
		t.Error("expected error for missing job")
	}
}

func TestNormalizeWorkingDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"D:", `D:\`},
		{"C:", `C:\`},
		{`D:\Code`, `D:\Code`},
		{".", "."},
	}

	for _, tt := range tests {
		got := normalizeWorkingDir(tt.input)
		if got != tt.want {
			t.Errorf("normalizeWorkingDir(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

