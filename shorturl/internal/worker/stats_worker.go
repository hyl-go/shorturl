package worker

import (
	"context"
	"database/sql"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const TypeStatsAggregateHour = "stats:aggregate:hour"

// StatsWorker 按小时将 access_log 聚合写入 access_stats。
type StatsWorker struct {
	DB sqlx.SqlConn
}

func NewStatsWorker(db sqlx.SqlConn) *StatsWorker {
	return &StatsWorker{DB: db}
}

// AggregateHour 聚合「上一完整小时」的访问日志。
func (w *StatsWorker) AggregateHour(ctx context.Context, _ *asynq.Task) error {
	now := time.Now()
	end := now.Truncate(time.Hour)
	start := end.Add(-time.Hour)

	type aggRow struct {
		Surl string `db:"surl"`
		Pv   int64  `db:"pv"`
		Uv   int64  `db:"uv"`
		Dm   int64  `db:"dm"`
		Dd   int64  `db:"dd"`
	}
	query := `SELECT surl,
COALESCE(COUNT(*), 0) AS pv,
COALESCE(COUNT(DISTINCT NULLIF(TRIM(ip), '')), 0) AS uv,
COALESCE(SUM(CASE WHEN device_type = 'mobile' THEN 1 ELSE 0 END), 0) AS dm,
COALESCE(SUM(CASE WHEN device_type <> 'mobile' OR device_type IS NULL OR device_type = '' THEN 1 ELSE 0 END), 0) AS dd
FROM access_log
WHERE access_time >= ? AND access_time < ?
GROUP BY surl`

	var rows []aggRow
	if err := w.DB.QueryRowsCtx(ctx, &rows, query, start, end); err != nil {
		return err
	}

	statDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	hourVal := int64(start.Hour())

	insert := `INSERT INTO access_stats (surl, date, hour, pv, uv, device_mobile, device_desktop, top_referer, top_country)
VALUES (?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE pv=VALUES(pv), uv=VALUES(uv), device_mobile=VALUES(device_mobile), device_desktop=VALUES(device_desktop)`

	for _, row := range rows {
		_, err := w.DB.ExecCtx(ctx, insert,
			row.Surl,
			statDate,
			hourVal,
			row.Pv,
			row.Uv,
			row.Dm,
			row.Dd,
			sql.NullString{Valid: false},
			sql.NullString{Valid: false},
		)
		if err != nil {
			logx.Errorf("access_stats upsert failed surl=%s: %v", row.Surl, err)
			return err
		}
	}

	logx.Infof("stats aggregate hour done window=[%s,%s) rows=%d", start.Format(time.RFC3339), end.Format(time.RFC3339), len(rows))
	return nil
}
