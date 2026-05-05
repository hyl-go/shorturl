package surlfilter

import (
	"context"
	"fmt"
	"strings"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// Cuckoo 基于 RedisBloom 的 Cuckoo Filter（命令 CF.*），支持 CF.DEL 删除元素。
// 需 Redis Stack 或安装 RedisBloom 模块；详见 https://redis.io/docs/stack/bloom/
type Cuckoo struct {
	rdb *goredis.Client
	key string
}

// NewCuckoo 创建并确保已 CF.RESERVE（若 key 不存在）。
func NewCuckoo(rdb *goredis.Client, key string, capacity int64) (*Cuckoo, error) {
	if rdb == nil {
		return nil, goredis.ErrClosed
	}
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	c := &Cuckoo{rdb: rdb, key: key}
	if err := c.ensureReserved(ctx, capacity); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Cuckoo) ensureReserved(ctx context.Context, capacity int64) error {
	_, err := c.rdb.Do(ctx, "CF.INFO", c.key).Result()
	if err == nil {
		return nil
	}
	// key 不存在或类型不对时 INFO 失败，尝试新建 Cuckoo（需 RedisBloom）
	_, err = c.rdb.Do(ctx, "CF.RESERVE", c.key, capacity).Result()
	return err
}

func (c *Cuckoo) Add(ctx context.Context, item []byte) error {
	if len(item) == 0 {
		return nil
	}
	_, err := c.rdb.Do(ctx, "CF.ADD", c.key, string(item)).Result()
	if err == nil {
		return nil
	}
	// 已存在时部分版本返回错误，视为幂等成功
	if strings.Contains(strings.ToLower(err.Error()), "exist") {
		return nil
	}
	return err
}

func (c *Cuckoo) Remove(ctx context.Context, item []byte) error {
	if len(item) == 0 {
		return nil
	}
	n, err := cfDelCount(c.rdb.Do(ctx, "CF.DEL", c.key, string(item)).Result())
	if err != nil {
		return err
	}
	if n == 0 {
		logx.Infof("cuckoo CF.DEL miss key=%s item=%q", c.key, string(item))
	}
	return nil
}

func (c *Cuckoo) Exists(ctx context.Context, item []byte) (bool, error) {
	if len(item) == 0 {
		return false, nil
	}
	return cfExistsBool(c.rdb.Do(ctx, "CF.EXISTS", c.key, string(item)).Result())
}

// RedisBloom 在不同版本/协议下 CF.EXISTS 可能返回 int64(0/1) 或 bool。
func cfExistsBool(res any, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	switch v := res.(type) {
	case int64:
		return v == 1, nil
	case int:
		return v == 1, nil
	case bool:
		return v, nil
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("CF.EXISTS: unexpected reply type %T", v)
	}
}

// CF.DEL 返回删除条数，部分栈可能用 bool 表示是否删除。
func cfDelCount(res any, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	switch v := res.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("CF.DEL: unexpected reply type %T", v)
	}
}

func (c *Cuckoo) Warmup(ctx context.Context, items []string) error {
	for _, s := range items {
		if s == "" {
			continue
		}
		if err := c.Add(ctx, []byte(s)); err != nil {
			logx.Errorf("cuckoo warmup add %q: %v", s, err)
		}
	}
	return nil
}
