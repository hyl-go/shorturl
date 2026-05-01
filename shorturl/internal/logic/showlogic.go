package logic

import (
	"context"
	"database/sql"
	"errors"
	"shorturl/internal/svc"
	"shorturl/internal/types"
	"shorturl/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type ShowLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewShowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShowLogic {
	return &ShowLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ShowLogic) Show(req *types.ShowRequest) (resp *types.ShowResponse, err error) {
	// 布隆过滤器前置拦截：不存在则 100% 不存在，直接 404，防止缓存穿透
	exists, err := l.svcCtx.Filter.Exists([]byte(req.ShortURL))
	if err != nil {
		logx.Errorw("Bloom Exists failed", logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}
	if !exists {
		return nil, errors.New("404")
	}
	// 查询短链接对应的长链接
	urlMap, err := l.svcCtx.ShortUrlMapModel.FindOneBySurl(l.ctx, sql.NullString{Valid: true, String: req.ShortURL})
	if err != nil {
		if err == model.ErrNotFound {
			// 布隆过滤器假正例，DB 里实际不存在
			return nil, errors.New("404")
		}
		logx.Errorw("ShortUrlMapModel.FindOneBySurl Failed", logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}
	return &types.ShowResponse{LongURL: urlMap.Lurl.String}, nil
}
