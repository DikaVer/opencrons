package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestModelValue(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"sonnet", false},
		{"opus", false},
		{"haiku", false},
		{"claude-sonnet-4-6", false},
		{"claude-opus-4-6", false},
		{"claude-haiku-4-5-20251001", false},
		{"xyz", true},
		{"gpt-4", true},
		{"", true},
		{"SONNET", true},
	}
	for _, tt := range tests {
		v := NewModelValue("")
		err := v.Set(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ModelValue.Set(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if err == nil && v.String() != tt.input {
			t.Errorf("ModelValue.Set(%q): String()=%q, want %q", tt.input, v.String(), tt.input)
		}
	}
	if v := NewModelValue("sonnet"); v.Type() != "model" {
		t.Errorf("ModelValue.Type() = %q, want %q", v.Type(), "model")
	}
}

func TestEffortValue(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"low", false},
		{"medium", false},
		{"high", false},
		{"max", false},
		{"banana", true},
		{"", true},
		{"HIGH", true},
		{"extreme", true},
	}
	for _, tt := range tests {
		v := NewEffortValue("")
		err := v.Set(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("EffortValue.Set(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if err == nil && v.String() != tt.input {
			t.Errorf("EffortValue.Set(%q): String()=%q, want %q", tt.input, v.String(), tt.input)
		}
	}
	if v := NewEffortValue(""); v.Type() != "effort" {
		t.Errorf("EffortValue.Type() = %q, want %q", v.Type(), "effort")
	}
}

func TestJobNameValue(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"my-job", false},
		{"test_123", false},
		{"SimpleJob", false},
		{"a", false},
		{"has space", true},
		{"special!", true},
		{"path/traversal", true},
		{"", true},
	}
	for _, tt := range tests {
		v := NewJobNameValue("")
		err := v.Set(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("JobNameValue.Set(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if err == nil && v.String() != tt.input {
			t.Errorf("JobNameValue.Set(%q): String()=%q, want %q", tt.input, v.String(), tt.input)
		}
	}
	if v := NewJobNameValue(""); v.Type() != "name" {
		t.Errorf("JobNameValue.Type() = %q, want %q", v.Type(), "name")
	}
}

func TestCronValue(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"* * * * *", false},
		{"0 2 * * *", false},
		{"*/5 * * * *", false},
		{"0 0 1 1 *", false},
		{"bad", true},
		{"", true},
		{"* * *", true},
		{"60 * * * *", true},
	}
	for _, tt := range tests {
		v := NewCronValue("")
		err := v.Set(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("CronValue.Set(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if err == nil && v.String() != tt.input {
			t.Errorf("CronValue.Set(%q): String()=%q, want %q", tt.input, v.String(), tt.input)
		}
	}
	if v := NewCronValue(""); v.Type() != "cron" {
		t.Errorf("CronValue.Type() = %q, want %q", v.Type(), "cron")
	}
}

func TestTimeoutValue(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		wantVal int
	}{
		{"300", false, 300},
		{"1", false, 1},
		{"3600", false, 3600},
		{"0", true, 0},
		{"-1", true, 0},
		{"abc", true, 0},
		{"", true, 0},
		{"1.5", true, 0},
	}
	for _, tt := range tests {
		v := NewTimeoutValue(300)
		err := v.Set(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("TimeoutValue.Set(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if err == nil {
			if v.Int() != tt.wantVal {
				t.Errorf("TimeoutValue.Set(%q): Int()=%d, want %d", tt.input, v.Int(), tt.wantVal)
			}
			if v.String() != strconv.Itoa(tt.wantVal) {
				t.Errorf("TimeoutValue.Set(%q): String()=%q, want %q", tt.input, v.String(), strconv.Itoa(tt.wantVal))
			}
		}
	}
	if v := NewTimeoutValue(300); v.Type() != "seconds" {
		t.Errorf("TimeoutValue.Type() = %q, want %q", v.Type(), "seconds")
	}
	// Default value preserved before Set
	if v := NewTimeoutValue(300); v.Int() != 300 {
		t.Errorf("NewTimeoutValue(300).Int() = %d, want 300", v.Int())
	}
}

func TestDirValue(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist", "xyz")

	tests := []struct {
		input   string
		wantErr bool
	}{
		{tmpDir, false},
		{nonExistent, true},
		{"", true},
	}
	for _, tt := range tests {
		v := NewDirValue("")
		err := v.Set(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("DirValue.Set(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if err == nil && v.String() != tt.input {
			t.Errorf("DirValue.Set(%q): String()=%q, want %q", tt.input, v.String(), tt.input)
		}
	}

	// Test with a file (not a directory)
	f, err := os.CreateTemp("", "flagtest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	v := NewDirValue("")
	if err := v.Set(f.Name()); err == nil {
		t.Errorf("DirValue.Set(file) should fail, got nil")
	}

	if v := NewDirValue(""); v.Type() != "directory" {
		t.Errorf("DirValue.Type() = %q, want %q", v.Type(), "directory")
	}
}
