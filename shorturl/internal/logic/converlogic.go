package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"shorturl/internal/crawler"
	"shorturl/model"
	"shorturl/pkg/base62"
	"shorturl/pkg/connect"
	"shorturl/pkg/md5"
	"shorturl/pkg/urlTool"
	"time"

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
	// 可达性校验：未开 AI 时用轻量 GET 探测；开 AI 时由网页抓取承担校验，避免重复完整 GET 拉长耗时与超时
	if !req.EnableAI {
		if ok := connect.Get(req.LongUrl); !ok {
			return nil, errors.New("无效的连接")
		}
	} else if !isAllowedHTTPURL(req.LongUrl) {
		return nil, errors.New("无效的连接")
	}
	// 给长链接生成md5
	md5Value := md5.Sum([]byte(req.LongUrl))
	u, err := l.svcCtx.ShortUrlMapModel.FindOneByMd5(l.ctx, sql.NullString{String: md5Value, Valid: true})
	if err == nil && u.IsDel != 0 {
		// 同 URL 曾被软删：允许重新生成新短链
		err = model.ErrNotFound
	}
	if err == nil {
		// 已存在，直接返回已有短链接（幂等）
		resp := &types.ConvertResponse{
			ShortUrl:     l.svcCtx.Config.ShortDomain + "/" + u.Surl.String,
			Category:     normalizeCategoryDisplay(u.Category.String),
			SafetyStatus: getSafetyLevelString(u.SafetyStatus),
		}
		if len(u.AiSuggestions) > 0 {
			_ = json.Unmarshal(u.AiSuggestions, &resp.AiSuggestions)
		}
		if u.ExpireAt.Valid {
			resp.ExpireAt = formatLocalDateTime(u.ExpireAt.Time)
		}
		return resp, nil
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
	// 处理自定义短链或自动生成
	var short string
	if req.CustomShortURL != "" {
		// 用户自定义短链：校验合法性
		if _, ok := l.svcCtx.ShortUrlBlackList[req.CustomShortURL]; ok {
			return nil, errors.New("自定义短链接包含敏感词，请更换")
		}
		_, err := l.svcCtx.ShortUrlMapModel.FindOneBySurl(l.ctx, sql.NullString{String: req.CustomShortURL, Valid: true})
		if err == nil {
			return nil, errors.New("自定义短链接已被占用，请更换")
		}
		if err != model.ErrNotFound {
			logx.Errorw("ShortUrlMapModel.FindOneBySurl failed", logx.LogField{Key: "surl", Value: err.Error()})
			return nil, err
		}
		short = req.CustomShortURL
	} else {
		// 自动生成：发号器 + Base62 + 黑名单过滤
		for {
			seq, err := l.svcCtx.Sequence.Next()
			if err != nil {
				logx.Errorw("Sequence.Next failed", logx.LogField{Key: "err", Value: err.Error()})
				return nil, err
			}
			logx.Infow("sequence next", logx.LogField{Key: "seq", Value: seq})
			short = base62.IntToString(seq)
			if _, ok := l.svcCtx.ShortUrlBlackList[short]; !ok {
				break
			}
		}
	}
	expireAt, err := resolveExpireAt(req)
	if err != nil {
		return nil, err
	}

	var aiResultCategory sql.NullString
	var aiSafetyStatus uint64
	var aiTitle sql.NullString
	var aiDesc sql.NullString
	var aiSuggestions []byte
	if req.EnableAI {
		pageInfo, fetchErr := l.svcCtx.Fetcher.Fetch(l.ctx, req.LongUrl)
		if fetchErr != nil {
			logx.Errorf("fetch page failed: %v", fetchErr)
			pageInfo = &crawler.PageInfo{}
		}
		analysis, aiErr := l.svcCtx.AIFactory.AnalyzeWithFallback(
			l.ctx,
			l.svcCtx.Config.AI.Provider,
			req.LongUrl,
			pageInfo.Title,
			pageInfo.Description,
		)
		fmt.Printf("analysis====》%v\n", analysis)
		if aiErr != nil {
			logx.Errorf("analyze url failed: %v", aiErr)
		} else if analysis != nil {
			if analysis.SafetyLevel == "dangerous" {
				return nil, fmt.Errorf("该链接存在安全风险，拒绝生成短链：%s", analysis.SafetyReason)
			}
			if strings.TrimSpace(analysis.Category) != "" {
				aiResultCategory = sql.NullString{String: strings.TrimSpace(analysis.Category), Valid: true}
			}
			aiSafetyStatus = getSafetyStatus(analysis.SafetyLevel)
			aiTitle = sql.NullString{String: analysis.PageTitle, Valid: analysis.PageTitle != ""}
			aiDesc = sql.NullString{String: analysis.PageDesc, Valid: analysis.PageDesc != ""}
			if len(analysis.Suggestions) > 0 {
				aiSuggestions, _ = json.Marshal(analysis.Suggestions)
			}
		}
	}
	if !aiResultCategory.Valid || strings.TrimSpace(aiResultCategory.String) == "" {
		aiResultCategory = sql.NullString{String: "其他", Valid: true}
	}

	if _, err := l.svcCtx.ShortUrlMapModel.Insert(
		l.ctx,
		&model.ShortUrlMap{
			Lurl:            sql.NullString{String: req.LongUrl, Valid: true},
			Md5:             sql.NullString{String: md5Value, Valid: true},
			Surl:            sql.NullString{String: short, Valid: true},
			ExpireAt:        expireAt,
			Category:        aiResultCategory,
			SafetyStatus:    aiSafetyStatus,
			PageTitle:       aiTitle,
			PageDescription: aiDesc,
			AiSuggestions:   aiSuggestions,
		}); err != nil {
		logx.Errorw("ShortUrlMapModel.Insert failed", logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}
	err = l.svcCtx.Filter.Add([]byte(short))
	if err != nil {
		logx.Errorw("Bloom Add failed", logx.LogField{Key: "err", Value: err.Error()})
	}
	shortUrl := l.svcCtx.Config.ShortDomain + "/" + short
	resp = &types.ConvertResponse{
		ShortUrl:     shortUrl,
		Category:     normalizeCategoryDisplay(aiResultCategory.String),
		SafetyStatus: getSafetyLevelString(aiSafetyStatus),
	}
	if len(aiSuggestions) > 0 {
		_ = json.Unmarshal(aiSuggestions, &resp.AiSuggestions)
	}
	if expireAt.Valid {
		resp.ExpireAt = formatLocalDateTime(expireAt.Time)
	}
	return resp, nil
}

func getSafetyStatus(level string) uint64 {
	switch level {
	case "dangerous":
		return 2
	case "suspicious":
		return 1
	default:
		return 0
	}
}

func getSafetyLevelString(level uint64) string {
	switch level {
	case 2:
		return "dangerous"
	case 1:
		return "suspicious"
	default:
		return "safe"
	}
}

func isAllowedHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		return true
	default:
		return false
	}
}

// resolveExpireAt 优先级：ExpirePreset → 相对时长 → ExpireAt(RFC3339)。
func resolveExpireAt(req *types.ConverRequest) (sql.NullTime, error) {
	preset := strings.TrimSpace(strings.ToLower(req.ExpirePreset))
	switch preset {
	case "30m":
		return sql.NullTime{Time: time.Now().Add(30 * time.Minute), Valid: true}, nil
	case "1h":
		return sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}, nil
	case "1d":
		return sql.NullTime{Time: time.Now().Add(24 * time.Hour), Valid: true}, nil
	case "7d":
		return sql.NullTime{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true}, nil
	case "none", "never":
		return sql.NullTime{}, nil
	case "":
	default:
		return sql.NullTime{}, fmt.Errorf("无效的过期预设 %q", req.ExpirePreset)
	}

	if req.ExpireAfterValue > 0 {
		if strings.TrimSpace(req.ExpireAfterUnit) == "" {
			return sql.NullTime{}, errors.New("填写了过期时长时请同时选择单位（分钟、小时、天等）")
		}
		t, err := addExpireFromNow(req.ExpireAfterValue, req.ExpireAfterUnit)
		if err != nil {
			return sql.NullTime{}, err
		}
		return sql.NullTime{Time: t, Valid: true}, nil
	}
	if req.ExpireAfterValue < 0 {
		return sql.NullTime{}, errors.New("过期时长不能为负数")
	}

	if strings.TrimSpace(req.ExpireAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpireAt))
		if err != nil {
			return sql.NullTime{}, errors.New("过期时间解析失败：请使用 RFC3339，或改用快捷/相对时间选项")
		}
		return sql.NullTime{Time: t, Valid: true}, nil
	}

	return sql.NullTime{}, nil
}

func addExpireFromNow(value int64, unitRaw string) (time.Time, error) {
	u := strings.ToLower(strings.TrimSpace(unitRaw))
	if value <= 0 {
		return time.Time{}, errors.New("过期时长须为正数")
	}
	v := int(value)
	now := time.Now()
	switch u {
	case "minute", "minutes":
		return now.Add(time.Duration(value) * time.Minute), nil
	case "hour", "hours":
		return now.Add(time.Duration(value) * time.Hour), nil
	case "day", "days":
		return now.AddDate(0, 0, v), nil
	case "week", "weeks":
		return now.AddDate(0, 0, v*7), nil
	case "month", "months":
		return now.AddDate(0, v, 0), nil
	case "year", "years":
		return now.AddDate(v, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("不支持的过期单位 %q，可选：minute hour day week month year", unitRaw)
	}
}
