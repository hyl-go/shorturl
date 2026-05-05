package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"shorturl/internal/svc"
	"shorturl/internal/types"
	"shorturl/internal/worker"
	"shorturl/model"
	"strings"
	"time"

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
	exists, err := l.svcCtx.Filter.Exists(l.ctx, []byte(req.ShortURL))
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
	if urlMap.IsDel != 0 {
		return nil, errors.New("404")
	}
	// 检查短链接是否已过期
	if urlMap.ExpireAt.Valid && time.Now().After(urlMap.ExpireAt.Time) {
		return nil, errors.New("链接已过期")
	}

	task := &worker.AccessLogTask{
		Surl:       req.ShortURL,
		AccessTime: time.Now(),
		IP:         req.IP,
		Country:    req.Country,
		City:       req.Region,
		UserAgent:  req.UserAgent,
		Referer:    req.Referer,
	}
	task.DeviceType, task.OS, task.Browser = parseUserAgent(req.UserAgent)
	if err := l.svcCtx.LogWorker.Enqueue(task); err != nil {
		logx.Errorf("enqueue access log failed: %v", err)
	}

	if task.IP != "" {
		day := time.Now().Format("2006-01-02")
		hllKey := fmt.Sprintf("stats:uv:%s:%s", req.ShortURL, day)
		if _, err := l.svcCtx.Redis.PfaddCtx(l.ctx, hllKey, task.IP); err != nil {
			logx.Errorf("redis pfadd uv failed: %v", err)
		}
		if err := l.svcCtx.Redis.ExpireCtx(l.ctx, hllKey, 7*24*3600); err != nil {
			logx.Errorf("redis expire uv key failed: %v", err)
		}
	}

	return &types.ShowResponse{LongURL: urlMap.Lurl.String}, nil
}

func parseUserAgent(ua string) (deviceType, osName, browser string) {
	lower := strings.ToLower(ua)
	deviceType = "desktop"
	switch {
	case strings.Contains(lower, "mobile"):
		deviceType = "mobile"
	case strings.Contains(lower, "tablet"):
		deviceType = "tablet"
	case strings.Contains(lower, "bot"):
		deviceType = "bot"
	}

	osName = "unknown"
	switch {
	case strings.Contains(lower, "windows"):
		osName = "windows"
	case strings.Contains(lower, "mac os"):
		osName = "macos"
	case strings.Contains(lower, "linux"):
		osName = "linux"
	case strings.Contains(lower, "android"):
		osName = "android"
	case strings.Contains(lower, "iphone"), strings.Contains(lower, "ios"):
		osName = "ios"
	}

	browser = "unknown"
	switch {
	case strings.Contains(lower, "edg/"):
		browser = "edge"
	case strings.Contains(lower, "chrome/"):
		browser = "chrome"
	case strings.Contains(lower, "firefox/"):
		browser = "firefox"
	case strings.Contains(lower, "safari/"):
		browser = "safari"
	}

	return deviceType, osName, browser
}
