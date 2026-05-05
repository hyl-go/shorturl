package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"shorturl/internal/svc"
	"shorturl/internal/types"
	"shorturl/model"
	"shorturl/pkg/connect"
	"shorturl/pkg/md5"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLinkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateLinkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLinkLogic {
	return &UpdateLinkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateLinkLogic) UpdateLink(req *types.LinkUpdateRequest) (*types.LinkUpdateResponse, error) {
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
		return nil, errors.New("链接已删除")
	}

	newLong := strings.TrimSpace(req.LongURL)
	if newLong != "" && newLong != strings.TrimSpace(row.Lurl.String) {
		probe := connect.Probe(l.ctx, l.svcCtx.UserURLProbe, newLong)
		if !probe.IsValidURL() {
			if probe.Status == connect.ProbeRejected {
				return nil, errors.New("长链接不可达或已失效")
			}
			return nil, errors.New("长链接校验失败，请稍后重试")
		}
		newMd5Val := md5.Sum([]byte(newLong))
		dup, errMd5 := l.svcCtx.ShortUrlMapModel.FindOneByMd5(l.ctx, sql.NullString{String: newMd5Val, Valid: true})
		if errMd5 == nil && dup.Id != row.Id && dup.IsDel == 0 {
			return nil, errors.New("该长链接已被其它有效短链占用")
		}
		if errMd5 != nil && errMd5 != model.ErrNotFound {
			return nil, errMd5
		}
		row.Lurl = sql.NullString{String: newLong, Valid: true}
		row.Md5 = sql.NullString{String: newMd5Val, Valid: true}
	}

	cat := strings.TrimSpace(req.Category)
	row.Category = sql.NullString{String: normalizeCategoryDisplay(cat), Valid: true}

	expireAt, expireTouched, err := expirePatchFromLinkUpdate(req)
	if err != nil {
		return nil, err
	}
	if expireTouched {
		row.ExpireAt = expireAt
	}

	if err := l.svcCtx.ShortUrlMapModel.Update(l.ctx, row); err != nil {
		logx.Errorw("ShortUrlMapModel.Update failed", logx.LogField{Key: "id", Value: req.Id}, logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}

	updated, err := l.svcCtx.ShortUrlMapModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	item := shortURLRowToListItem(*updated, l.svcCtx.Config.ShortDomain)
	return &types.LinkUpdateResponse{Item: item}, nil
}

func expirePatchFromLinkUpdate(req *types.LinkUpdateRequest) (sql.NullTime, bool, error) {
	if req.NoExpire {
		return sql.NullTime{}, true, nil
	}
	hasAny := strings.TrimSpace(req.ExpirePreset) != "" || req.ExpireAfterValue > 0 || strings.TrimSpace(req.ExpireAt) != ""
	if !hasAny {
		return sql.NullTime{}, false, nil
	}
	sub := types.ConverRequest{
		ExpirePreset:     req.ExpirePreset,
		ExpireAfterValue: req.ExpireAfterValue,
		ExpireAfterUnit:  req.ExpireAfterUnit,
		ExpireAt:         req.ExpireAt,
	}
	t, err := resolveExpireAt(&sub)
	return t, true, err
}
