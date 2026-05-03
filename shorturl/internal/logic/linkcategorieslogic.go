package logic

import (
	"context"

	"shorturl/internal/svc"
	"shorturl/internal/types"
)

type LinkCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLinkCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LinkCategoriesLogic {
	return &LinkCategoriesLogic{ctx: ctx, svcCtx: svcCtx}
}

// LinkCategories 返回当前库中未删除链接的去重分类（空分类视为「其他」），供管理端筛选下拉使用。
func (l *LinkCategoriesLogic) LinkCategories() (*types.LinkCategoriesResponse, error) {
	type row struct {
		Cat string `db:"cat"`
	}
	query := `SELECT DISTINCT
CASE WHEN category IS NULL OR TRIM(category) = '' THEN '其他' ELSE TRIM(category) END AS cat
FROM short_url_map
WHERE is_del = 0
ORDER BY cat`
	var rows []row
	if err := l.svcCtx.DbConn.QueryRowsCtx(l.ctx, &rows, query); err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Cat
	}
	return &types.LinkCategoriesResponse{Categories: out}, nil
}
