// flagtypes.go defines custom pflag.Value types for CLI flag validation.
//
// Each type validates input at parse time, producing clear error messages and
// descriptive type names in --help output. The canonical ValidModels and
// ValidEfforts maps serve as the single source of truth across the application.
package ui

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ValidModels is the canonical set of accepted model identifiers.
// Treat as read-only; do not add or remove entries at runtime.
var ValidModels = map[string]bool{
	"sonnet": true, "opus": true, "haiku": true,
	"claude-sonnet-4-6": true, "claude-opus-4-6": true, "claude-haiku-4-5-20251001": true,
}

// ValidEfforts is the canonical set of accepted effort levels.
// Treat as read-only; do not add or remove entries at runtime.
var ValidEfforts = map[string]bool{
	"low": true, "medium": true, "high": true, "max": true,
}

// ValidContainers is the canonical set of accepted container runtimes.
// Treat as read-only; do not add or remove entries at runtime.
var ValidContainers = map[string]bool{
	"docker": true, "podman": true,
}

// sortedKeys returns the sorted keys of a map for deterministic error messages.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- ModelValue ---

// ModelValue validates that the flag value is a recognized Claude model.
type ModelValue struct {
	val string
}

func NewModelValue(def string) *ModelValue { return &ModelValue{val: def} }
func (v *ModelValue) String() string       { return v.val }
func (v *ModelValue) Type() string         { return "model" }

func (v *ModelValue) Set(s string) error {
	if !ValidModels[s] {
		return fmt.Errorf("invalid model %q (valid: %s)", s, strings.Join(sortedKeys(ValidModels), ", "))
	}
	v.val = s
	return nil
}

// --- EffortValue ---

// EffortValue validates that the flag value is a recognized effort level.
type EffortValue struct {
	val string
}

func NewEffortValue(def string) *EffortValue { return &EffortValue{val: def} }
func (v *EffortValue) String() string        { return v.val }
func (v *EffortValue) Type() string          { return "effort" }

func (v *EffortValue) Set(s string) error {
	if !ValidEfforts[s] {
		return fmt.Errorf("invalid effort %q (valid: %s)", s, strings.Join(sortedKeys(ValidEfforts), ", "))
	}
	v.val = s
	return nil
}

// --- JobNameValue ---

// JobNameValue validates job names using the shared ValidateJobName function.
type JobNameValue struct {
	val string
}

func NewJobNameValue(def string) *JobNameValue { return &JobNameValue{val: def} }
func (v *JobNameValue) String() string         { return v.val }
func (v *JobNameValue) Type() string           { return "name" }

func (v *JobNameValue) Set(s string) error {
	if err := ValidateJobName(s); err != nil {
		return err
	}
	v.val = s
	return nil
}

// --- CronValue ---

// CronValue validates 5-field cron expressions using the shared CronParser.
type CronValue struct {
	val string
}

func NewCronValue(def string) *CronValue { return &CronValue{val: def} }
func (v *CronValue) String() string      { return v.val }
func (v *CronValue) Type() string        { return "cron" }

func (v *CronValue) Set(s string) error {
	if err := ValidateCron(s); err != nil {
		return err
	}
	v.val = s
	return nil
}

// --- TimeoutValue ---

// TimeoutValue validates that the flag value is a positive integer (seconds).
type TimeoutValue struct {
	val int
}

func NewTimeoutValue(def int) *TimeoutValue { return &TimeoutValue{val: def} }
func (v *TimeoutValue) String() string      { return strconv.Itoa(v.val) }
func (v *TimeoutValue) Type() string        { return "seconds" }
func (v *TimeoutValue) Int() int            { return v.val }

func (v *TimeoutValue) Set(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: must be an integer", s)
	}
	if n <= 0 {
		return fmt.Errorf("invalid timeout %q: must be a positive integer", s)
	}
	v.val = n
	return nil
}

// --- DirValue ---

// DirValue validates that the flag value is an existing directory.
type DirValue struct {
	val string
}

func NewDirValue(def string) *DirValue { return &DirValue{val: def} }
func (v *DirValue) String() string     { return v.val }
func (v *DirValue) Type() string       { return "directory" }

func (v *DirValue) Set(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("directory path is required")
	}
	info, err := os.Stat(s)
	if err != nil {
		return fmt.Errorf("cannot access directory %q: %w", s, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", s)
	}
	v.val = s
	return nil
}

// --- ContainerValue ---

// ContainerValue validates that the flag value is a recognized container runtime.
type ContainerValue struct {
	val string
}

func NewContainerValue(def string) *ContainerValue { return &ContainerValue{val: def} }
func (v *ContainerValue) String() string           { return v.val }
func (v *ContainerValue) Type() string             { return "container" }

func (v *ContainerValue) Set(s string) error {
	if !ValidContainers[s] {
		return fmt.Errorf("invalid container %q (valid: %s)", s, strings.Join(sortedKeys(ValidContainers), ", "))
	}
	v.val = s
	return nil
}
