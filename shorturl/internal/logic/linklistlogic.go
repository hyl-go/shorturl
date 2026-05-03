package logic

import (
	"context"

	"shorturl/internal/svc"
	"shorturl/internal/types"
)

type LinkListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLinkListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LinkListLogic {
	return &LinkListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LinkListLogic) LinkList(req *types.LinkListRequest) (*types.LinkListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	categoryFilter := req.Category

	total, err := l.svcCtx.ShortUrlMapModel.CountUndeleted(l.ctx, categoryFilter)
	if err != nil {
		return nil, err
	}

	offset := int((page - 1) * pageSize)
	limit := int(pageSize)
	rows, err := l.svcCtx.ShortUrlMapModel.FindPageUndeleted(l.ctx, categoryFilter, offset, limit)
	if err != nil {
		return nil, err
	}

	list := make([]types.LinkListItem, 0, len(rows))
	domain := l.svcCtx.Config.ShortDomain
	for _, row := range rows {
		list = append(list, shortURLRowToListItem(row, domain))
	}

	return &types.LinkListResponse{
		Total: total,
		List:  list,
	}, nil
}
