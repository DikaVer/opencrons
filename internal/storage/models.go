package storage

import "time"

// ExecutionLog represents a single job execution record.
type ExecutionLog struct {
	ID                   int64      `json:"id"`
	JobID                string     `json:"job_id"`
	JobName              string     `json:"job_name"`
	StartedAt            time.Time  `json:"started_at"`
	FinishedAt           *time.Time `json:"finished_at,omitempty"`
	ExitCode             *int       `json:"exit_code,omitempty"`
	StdoutPath           string     `json:"stdout_path"`
	StderrPath           string     `json:"stderr_path"`
	CostUSD              *float64   `json:"cost_usd,omitempty"`
	InputTokens          *int       `json:"input_tokens,omitempty"`
	OutputTokens         *int       `json:"output_tokens,omitempty"`
	CacheReadTokens      *int       `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens  *int       `json:"cache_creation_tokens,omitempty"`
	Status               string     `json:"status"`
	TriggerType          string     `json:"trigger_type"`
	ErrorMsg             string     `json:"error_msg,omitempty"`
}

// UsageSummary holds aggregated usage stats.
type UsageSummary struct {
	TotalRuns           int     `json:"total_runs"`
	TotalCostUSD        float64 `json:"total_cost_usd"`
	TotalInputTokens    int     `json:"total_input_tokens"`
	TotalOutputTokens   int     `json:"total_output_tokens"`
	TotalCacheRead      int     `json:"total_cache_read_tokens"`
	TotalCacheCreation  int     `json:"total_cache_creation_tokens"`
}
