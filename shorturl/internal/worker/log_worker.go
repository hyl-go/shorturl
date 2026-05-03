package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const TypeAccessLog = "access:log"

type AccessLogTask struct {
	Surl       string    `json:"surl"`
	AccessTime time.Time `json:"access_time"`
	IP         string    `json:"ip"`
	Country    string    `json:"country"`
	City       string    `json:"city"`
	UserAgent  string    `json:"user_agent"`
	DeviceType string    `json:"device_type"`
	OS         string    `json:"os"`
	Browser    string    `json:"browser"`
	Referer    string    `json:"referer"`
}

type LogWorker struct {
	Client *asynq.Client
	DB     sqlx.SqlConn
}

func (w *LogWorker) Enqueue(task *AccessLogTask) error {
	payload, _ := json.Marshal(task)
	t := asynq.NewTask(TypeAccessLog, payload)
	_, err := w.Client.Enqueue(t, asynq.MaxRetry(3))
	return err
}

func (w *LogWorker) HandleAccessLog(ctx context.Context, t *asynq.Task) error {
	var task AccessLogTask
	if err := json.Unmarshal(t.Payload(), &task); err != nil {
		return err
	}
	query := "insert into access_log (surl,access_time,ip,country,city,user_agent,device_type,os,browser,referer) values (?,?,?,?,?,?,?,?,?,?)"
	_, err := w.DB.ExecCtx(
		ctx,
		query,
		sql.NullString{String: task.Surl, Valid: task.Surl != ""},
		task.AccessTime,
		sql.NullString{String: task.IP, Valid: task.IP != ""},
		sql.NullString{String: task.Country, Valid: task.Country != ""},
		sql.NullString{String: task.City, Valid: task.City != ""},
		sql.NullString{String: task.UserAgent, Valid: task.UserAgent != ""},
		sql.NullString{String: task.DeviceType, Valid: task.DeviceType != ""},
		sql.NullString{String: task.OS, Valid: task.OS != ""},
		sql.NullString{String: task.Browser, Valid: task.Browser != ""},
		sql.NullString{String: task.Referer, Valid: task.Referer != ""},
	)
	return err
}
