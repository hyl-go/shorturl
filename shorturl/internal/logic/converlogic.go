package logic

import (
	"context"
	"database/sql"
	"errors"
	"shorturl/model"
	"shorturl/pkg/base62"
	"shorturl/pkg/connect"
	"shorturl/pkg/md5"
	"shorturl/pkg/urlTool"

	"shorturl/internal/svc"
	"shorturl/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConverLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConverLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConverLogic {
	return &ConverLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 转链，输入一个长链接输出一个短链接
func (l *ConverLogic) Conver(req *types.ConverRequest) (resp *types.ConvertResponse, err error) {
	// 输入的链接是否有效
	if ok := connect.Get(req.LongUrl); !ok {
		return nil, errors.New("无效的连接")
	}
	// 给长链接生成md5
	md5Value := md5.Sum([]byte(req.LongUrl))
	u, err := l.svcCtx.ShortUrlMapModel.FindOneByMd5(l.ctx, sql.NullString{String: md5Value, Valid: true})
	if err == nil {
		// 已存在，直接返回已有短链接（幂等）
		return &types.ConvertResponse{ShortUrl: l.svcCtx.Config.ShortDomain + "/" + u.Surl.String}, nil
	}
	if err != model.ErrNotFound {
		logx.Errorw("ShortUrlMapModel.FindOneByMd5 failed", logx.LogField{Key: "md5", Value: err.Error()})
		return nil, err
	}
	baseUrl, err := urlTool.GetBasePath(req.LongUrl)
	if err != nil {
		logx.Errorw("GetBasePath failed", logx.LogField{Key: "url", Value: req.LongUrl})
		return nil, err
	}
	_, err = l.svcCtx.ShortUrlMapModel.FindOneBySurl(l.ctx, sql.NullString{String: baseUrl, Valid: true})
	if err == nil {
		return nil, errors.New("该链接已经是一个短链接了")
	}
	if err != model.ErrNotFound {
		logx.Errorw("ShortUrlMapModel.FindOneBySurl failed", logx.LogField{Key: "url", Value: err.Error()})
		return nil, err
	}
	// 取号基于mysql实现发号器
	var short string
	for {
		seq, err := l.svcCtx.Sequence.Next()
		if err != nil {
			logx.Errorw("Sequence.Next failed", logx.LogField{Key: "err", Value: err.Error()})
			return nil, err
		}
		logx.Infow("sequence next", logx.LogField{Key: "seq", Value: seq})
		// 号码转62进制短链接
		short = base62.IntToString(seq)
		// 黑名单判断
		if _, ok := l.svcCtx.ShortUrlBlackList[short]; !ok {
			break
		}
	}
	if _, err := l.svcCtx.ShortUrlMapModel.Insert(
		l.ctx,
		&model.ShortUrlMap{
			Lurl: sql.NullString{String: req.LongUrl, Valid: true},
			Md5:  sql.NullString{String: md5Value, Valid: true},
			Surl: sql.NullString{String: short, Valid: true},
		}); err != nil {
		logx.Errorw("ShortUrlMapModel.Insert failed", logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}
	err = l.svcCtx.Filter.Add([]byte(short))
	if err != nil {
		logx.Errorw("Bloom Add failed", logx.LogField{Key: "err", Value: err.Error()})
	}
	shortUrl := l.svcCtx.Config.ShortDomain + "/" + short
	return &types.ConvertResponse{ShortUrl: shortUrl}, nil
}
