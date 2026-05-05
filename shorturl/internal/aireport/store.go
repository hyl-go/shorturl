package aireport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// JobStatus Redis 中异步报告任务状态。
type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
)

// JobRecord Redis JSON；MarkdownEdited 非空时前端优先展示编辑稿。
type JobRecord struct {
	Status         JobStatus `json:"status"`
	ShortURL     string `json:"shortURL"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	AIReportJSON string `json:"aiReportJson,omitempty"`
	MarkdownEdited string    `json:"markdownEdited,omitempty"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Store 任务状态与结果（24h TTL）。
type Store struct {
	Rdb *redis.Client
	TTL time.Duration
}

func (s *Store) key(id string) string {
	return KeyPrefix + strings.TrimSpace(id)
}

func (s *Store) ttl() time.Duration {
	if s.TTL > 0 {
		return s.TTL
	}
	return 24 * time.Hour
}

// CreatePending 创建任务记录（HTTP 入队前调用）。
func (s *Store) CreatePending(ctx context.Context, jobID, shortURL, start, end string) error {
	if s.Rdb == nil {
		return errors.New("redis client nil")
	}
	now := time.Now()
	rec := JobRecord{
		Status:    StatusPending,
		ShortURL:  shortURL,
		StartDate: start,
		EndDate:   end,
		CreatedAt: now,
		UpdatedAt: now,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.Rdb.Set(ctx, s.key(jobID), b, s.ttl()).Err()
}

func (s *Store) patch(ctx context.Context, jobID string, fn func(*JobRecord)) error {
	raw, err := s.Rdb.Get(ctx, s.key(jobID)).Bytes()
	if err != nil {
		return err
	}
	var rec JobRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return err
	}
	fn(&rec)
	rec.UpdatedAt = time.Now()
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.Rdb.Set(ctx, s.key(jobID), b, s.ttl()).Err()
}

// SetRunning Worker 开始处理前调用。
func (s *Store) SetRunning(ctx context.Context, jobID string) error {
	return s.patch(ctx, jobID, func(r *JobRecord) {
		r.Status = StatusRunning
		r.Error = ""
	})
}

// SetCompleted 写入 AI 报告 JSON（types.AIReport 序列化）。
func (s *Store) SetCompleted(ctx context.Context, jobID string, aiReportJSON string) error {
	return s.patch(ctx, jobID, func(r *JobRecord) {
		r.Status = StatusCompleted
		r.AIReportJSON = aiReportJSON
		r.Error = ""
	})
}

// SetFailed 任务彻底失败（极少：Redis/致命错误）；必要时仍可写入占位 aiReportJson。
func (s *Store) SetFailed(ctx context.Context, jobID string, errMsg string, fallbackReportJSON string) error {
	return s.patch(ctx, jobID, func(r *JobRecord) {
		r.Status = StatusFailed
		r.Error = errMsg
		if fallbackReportJSON != "" {
			r.AIReportJSON = fallbackReportJSON
		}
	})
}

// SetMarkdownEdited 管理端覆盖展示用 Markdown。
func (s *Store) SetMarkdownEdited(ctx context.Context, jobID string, md string) error {
	return s.patch(ctx, jobID, func(r *JobRecord) {
		r.MarkdownEdited = strings.TrimSpace(md)
	})
}

// Get 读取任务（轮询、更新 Markdown 用）。
func (s *Store) Get(ctx context.Context, jobID string) (*JobRecord, error) {
	raw, err := s.Rdb.Get(ctx, s.key(jobID)).Bytes()
	if err != nil {
		return nil, err
	}
	var rec JobRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// Delete 入队失败等场景下回收 pending 记录。
func (s *Store) Delete(ctx context.Context, jobID string) error {
	if s.Rdb == nil {
		return errors.New("redis client nil")
	}
	return s.Rdb.Del(ctx, s.key(jobID)).Err()
}
