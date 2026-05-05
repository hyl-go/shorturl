package worker

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"shorturl/model"
	"shorturl/pkg/surlfilter"
)

const TypeLinkGCPurge = "links:gc:purge"

const gcBatchSize = 500

// LinkGCWorker 物理删除「软删已过保留期」的行，释放 lurl/surl 唯一槽位。
// 删除前按批失效 short_url_map 在 Redis 中的 CachedConn 键，避免 FindOneBySurl 等仍命中旧缓存。
type LinkGCWorker struct {
	DB     sqlx.SqlConn
	Redis  *redis.Redis
	Filter surlfilter.Filter
}

func NewLinkGCWorker(db sqlx.SqlConn, r *redis.Redis, f surlfilter.Filter) *LinkGCWorker {
	return &LinkGCWorker{DB: db, Redis: r, Filter: f}
}

type tombstonePick struct {
	Id   uint64         `db:"id"`
	Lurl sql.NullString `db:"lurl"`
	Md5  sql.NullString `db:"md5"`
	Surl sql.NullString `db:"surl"`
}

func (w *LinkGCWorker) PurgeOldTombstones(ctx context.Context, task *asynq.Task) error {
	days := 90
	if len(task.Payload()) > 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(string(task.Payload()))); err == nil && v > 0 {
			days = v
		}
	}
	var totalDeleted int64
	for {
		var batch []tombstonePick
		q := `SELECT id, lurl, md5, surl FROM short_url_map
WHERE is_del = 1 AND update_at < DATE_SUB(NOW(), INTERVAL ? DAY) LIMIT ?`
		if err := w.DB.QueryRowsCtx(ctx, &batch, q, days, gcBatchSize); err != nil {
			logx.Errorf("link gc select batch failed: %v", err)
			return err
		}
		if len(batch) == 0 {
			break
		}
		if w.Redis != nil {
			for _, row := range batch {
				keys := model.ShortURLMapRowCacheKeys(row.Id, row.Lurl, row.Md5, row.Surl)
				if _, err := w.Redis.DelCtx(ctx, keys...); err != nil {
					logx.Errorf("link gc cache del id=%d: %v", row.Id, err)
				}
			}
		}
		ids := make([]uint64, len(batch))
		for i := range batch {
			ids[i] = batch[i].Id
		}
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		delQ := "DELETE FROM short_url_map WHERE id IN (" + placeholders + ")"
		args := make([]interface{}, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		res, err := w.DB.ExecCtx(ctx, delQ, args...)
		if err != nil {
			logx.Errorf("link gc delete batch failed: %v", err)
			return err
		}
		if w.Filter != nil {
			for _, row := range batch {
				if row.Surl.Valid && row.Surl.String != "" {
					if err := w.Filter.Remove(ctx, []byte(row.Surl.String)); err != nil {
						logx.Errorf("link gc filter remove surl=%s: %v", row.Surl.String, err)
					}
				}
			}
		}
		n, _ := res.RowsAffected()
		totalDeleted += n
	}
	logx.Infof("link gc purge done retention_days=%d total_rows=%d", days, totalDeleted)
	return nil
}
