package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ShortUrlMapModel = (*customShortUrlMapModel)(nil)

type (
	// ShortUrlMapModel is an interface to be customized, add more methods here,
	// and implement the added methods in customShortUrlMapModel.
	ShortUrlMapModel interface {
		shortUrlMapModel
		FindAllSurl(ctx context.Context) ([]string, error)
		// CountUndeleted categoryFilter 为空表示全部；「其他」包含 NULL / 空串
		CountUndeleted(ctx context.Context, categoryFilter string) (int64, error)
		FindPageUndeleted(ctx context.Context, categoryFilter string, offset, limit int) ([]ShortUrlMap, error)
	}

	customShortUrlMapModel struct {
		*defaultShortUrlMapModel
	}
)

// NewShortUrlMapModel returns a model for the database table.
func NewShortUrlMapModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ShortUrlMapModel {
	return &customShortUrlMapModel{
		defaultShortUrlMapModel: newShortUrlMapModel(conn, c, opts...),
	}
}

// FindAllSurl 查询所有未删除且未过期的短链接，用于启动时预热布隆过滤器
func (m *customShortUrlMapModel) FindAllSurl(ctx context.Context) ([]string, error) {
	query := fmt.Sprintf("select `surl` from %s where `is_del` = 0 and `surl` is not null and (`expire_at` is null or `expire_at` > now())", m.table)
	var surls []string
	err := m.QueryRowsNoCacheCtx(ctx, &surls, query)
	return surls, err
}

func (m *customShortUrlMapModel) CountUndeleted(ctx context.Context, categoryFilter string) (int64, error) {
	where := "`is_del` = 0"
	args := []interface{}{}
	cat := strings.TrimSpace(categoryFilter)
	if cat != "" {
		if cat == "其他" {
			where += " AND (IFNULL(TRIM(`category`),'') = '' OR `category` = '其他')"
		} else {
			where += " AND `category` = ?"
			args = append(args, cat)
		}
	}
	query := fmt.Sprintf("select count(1) from %s where %s", m.table, where)
	var total int64
	err := m.QueryRowNoCacheCtx(ctx, &total, query, args...)
	return total, err
}

func (m *customShortUrlMapModel) FindPageUndeleted(ctx context.Context, categoryFilter string, offset, limit int) ([]ShortUrlMap, error) {
	where := "`is_del` = 0"
	args := []interface{}{}
	cat := strings.TrimSpace(categoryFilter)
	if cat != "" {
		if cat == "其他" {
			where += " AND (IFNULL(TRIM(`category`),'') = '' OR `category` = '其他')"
		} else {
			where += " AND `category` = ?"
			args = append(args, cat)
		}
	}
	query := fmt.Sprintf(`select id, create_at, update_at, create_by, is_del, lurl, md5, surl, expire_at,
category, safety_status, page_title, page_description, ai_suggestions from %s
where %s order by id desc limit ? offset ?`, m.table, where)
	args = append(args, limit, offset)
	var rows []ShortUrlMap
	err := m.QueryRowsNoCacheCtx(ctx, &rows, query, args...)
	return rows, err
}
