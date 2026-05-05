package ai

import "context"

type AnalyzeResult struct {
	Suggestions  []string `json:"suggestions"`
	Category     string   `json:"category"`
	SafetyLevel  string   `json:"safety_level"`
	SafetyReason string   `json:"safety_reason"`
	PageTitle    string   `json:"page_title"`
	PageDesc     string   `json:"page_desc"`
}

type ReportResult struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Trends      []string `json:"trends"`
	Anomalies   []string `json:"anomalies"`
	Suggestions []string `json:"suggestions"`
	Markdown    string   `json:"markdown"`
}

type AIProvider interface {
	AnalyzeURL(ctx context.Context, url, title, desc string) (*AnalyzeResult, error)
	GenerateReport(ctx context.Context, statsData string) (*ReportResult, error)
	Name() string
}
