package logic

import (
	"context"
	"errors"

	"shorturl/internal/svc"
	"shorturl/internal/types"
	"shorturl/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteLinkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteLinkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteLinkLogic {
	return &DeleteLinkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteLinkLogic) DeleteLink(req *types.LinkDeleteRequest) (*types.LinkDeleteResponse, error) {
	if req.Id == 0 {
		return nil, errors.New("无效的链接 ID")
	}
	row, err := l.svcCtx.ShortUrlMapModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errors.New("链接不存在")
		}
		return nil, err
	}
	if row.IsDel != 0 {
		return &types.LinkDeleteResponse{Ok: true}, nil
	}
	row.IsDel = 1
	if err := l.svcCtx.ShortUrlMapModel.Update(l.ctx, row); err != nil {
		logx.Errorw("soft delete ShortUrlMap failed", logx.LogField{Key: "id", Value: req.Id}, logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}
	return &types.LinkDeleteResponse{Ok: true}, nil
}
