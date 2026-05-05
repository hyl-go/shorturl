package sequence

import (
	"context"
	"errors"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Redis 使用 Redis INCR 原子发号，避免 MySQL REPLACE INTO 热点。
// 首次使用前可通过 BootstrapFromSequenceTable 将计数器对齐到 `sequence` 表当前 MAX(id)。
type Redis struct {
	rdb *goredis.Client
	key string
}

// NewRedis 创建基于 INCR 的发号器；bootstrapConn 非空且 enableBootstrap 时，若 Redis 中 key 不存在则从 MySQL sequence 表拉一次 MAX(id) 作为初值。
func NewRedis(rdb *goredis.Client, key string, bootstrapConn sqlx.SqlConn, enableBootstrap bool) (*Redis, error) {
	if rdb == nil {
		return nil, errors.New("sequence redis: nil client")
	}
	if key == "" {
		key = "shorturl:sequence"
	}
	s := &Redis{rdb: rdb, key: key}
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	if enableBootstrap && bootstrapConn != nil {
		if err := s.bootstrapOnce(ctx, bootstrapConn); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Redis) bootstrapOnce(ctx context.Context, conn sqlx.SqlConn) error {
	_, err := s.rdb.Get(ctx, s.key).Result()
	if err == nil {
		return nil
	}
	if err != goredis.Nil {
		return err
	}
	var max uint64
	q := "SELECT COALESCE(MAX(id), 0) FROM `sequence`"
	if err := conn.QueryRowCtx(ctx, &max, q); err != nil {
		return fmt.Errorf("sequence bootstrap: %w", err)
	}
	if max == 0 {
		return nil
	}
	return s.rdb.Set(ctx, s.key, max, 0).Err()
}

func (s *Redis) Next() (uint64, error) {
	n, err := s.rdb.Incr(context.Background(), s.key).Uint64()
	if err != nil {
		return 0, err
	}
	return n, nil
}
