package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"shorturl/internal/crawler"
	"shorturl/model"
	"shorturl/pkg/base62"
	"shorturl/pkg/connect"
	"shorturl/pkg/md5"
	"shorturl/pkg/urlTool"
	"strings"
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

// Conver 转链流程（基线 + AI 扩展）：
//  1. 连通性：对 longURL 轻量 GET，非 2xx 视为无效（含开启 AI 时；其后 Fetcher 可能再次请求用于标题/描述）。
//  2. 同长链（MD5）已存在：区分 有效 / 已过期 / 已软删 —— 返回或 UPDATE 续约、复活（不得 INSERT，否则 unique(lurl) 冲突）。
//  3. 防套娃：长链基底路径若已是已有 surl，拒绝。
//  4. 生成 surl：自定义（黑名单 + 占用检查）或 Sequence+Base62（黑名单则循环重取）。
//  5. 过期：ExpirePreset / ExpireAfter* / ExpireAt(RFC3339) 解析；全无则 expire_at 为 NULL。
//  6. AI（可选）：爬取 + 分析，危险则拒绝写入。
//  7. Bloom.Add（先于 DB 写入，避免 Insert 成功但布隆未写入导致跳转永久 404）→ Insert → 响应。
func (l *ConverLogic) Conver(req *types.ConverRequest) (resp *types.ConvertResponse, err error) {
	if ok := connect.Get(l.ctx, l.svcCtx.UserURLProbe, req.LongUrl); !ok {
		return nil, errors.New("无效的连接")
	}

	md5Value := md5.Sum([]byte(req.LongUrl))
	u, err := l.svcCtx.ShortUrlMapModel.FindOneByMd5(l.ctx, sql.NullString{String: md5Value, Valid: true})
	if err != nil && err != model.ErrNotFound {
		logx.Errorw("ShortUrlMapModel.FindOneByMd5 failed", logx.LogField{Key: "md5", Value: err.Error()})
		return nil, err
	}
	if err == nil {
		now := time.Now()
		expired := u.ExpireAt.Valid && now.After(u.ExpireAt.Time)
		deleted := u.IsDel != 0

		switch {
		case !deleted && !expired:
			// 有效映射：幂等返回（忽略本次自定义短码、过期策略等）
			return l.rowToConvertResponse(u, linkReuseSameActive), nil
		case !deleted && expired:
			// 未删除但已过展示期：同记录续约 expire / 可选刷新 AI
			return l.persistConvertedRowUpdate(u, req, linkReuseRenewedExpired)
		default:
			// 已软删：同 lurl 无法再 INSERT，在同记录上复活
			return l.persistConvertedRowUpdate(u, req, linkReuseReactivatedDeleted)
		}
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

	aiResultCategory, aiSafetyStatus, aiTitle, aiDesc, aiSuggestions, err := l.mergeAIOrKeepRow(req, nil)
	if err != nil {
		return nil, err
	}
	if !aiResultCategory.Valid || strings.TrimSpace(aiResultCategory.String) == "" {
		aiResultCategory = sql.NullString{String: "其他", Valid: true}
	}

	if err := l.svcCtx.Filter.Add([]byte(short)); err != nil {
		logx.Errorw("Bloom Add failed", logx.LogField{Key: "err", Value: err.Error()})
		return nil, errors.New("短链索引写入失败，请稍后重试")
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
		if existing, ok := l.recoverFromDuplicateLurl(req.LongUrl, err); ok {
			return existing, nil
		}
		logx.Errorw("ShortUrlMapModel.Insert failed", logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}
	inserted := model.ShortUrlMap{
		Surl:            sql.NullString{String: short, Valid: true},
		ExpireAt:        expireAt,
		Category:        aiResultCategory,
		SafetyStatus:    aiSafetyStatus,
		PageTitle:       aiTitle,
		PageDescription: aiDesc,
		AiSuggestions:   aiSuggestions,
	}
	return l.rowToConvertResponse(&inserted, linkReuseInsertedNew), nil
}

const (
	linkReuseSameActive         = "same_active"
	linkReuseRenewedExpired     = "renewed_expired"
	linkReuseReactivatedDeleted = "reactivated_deleted"
	linkReuseInsertedNew        = "inserted_new"
)

func (l *ConverLogic) rowToConvertResponse(u *model.ShortUrlMap, reuse string) *types.ConvertResponse {
	resp := &types.ConvertResponse{
		ShortUrl:     l.svcCtx.Config.ShortDomain + "/" + u.Surl.String,
		Category:     normalizeCategoryDisplay(u.Category.String),
		SafetyStatus: getSafetyLevelString(u.SafetyStatus),
		LinkReuse:    reuse,
	}
	if len(u.AiSuggestions) > 0 {
		_ = json.Unmarshal(u.AiSuggestions, &resp.AiSuggestions)
	}
	if u.ExpireAt.Valid {
		resp.ExpireAt = formatLocalDateTime(u.ExpireAt.Time)
	}
	return resp
}

func (l *ConverLogic) persistConvertedRowUpdate(row *model.ShortUrlMap, req *types.ConverRequest, reuseTag string) (*types.ConvertResponse, error) {
	expireAt, err := resolveExpireAt(req)
	if err != nil {
		return nil, err
	}
	cat, safety, title, desc, sugBytes, err := l.mergeAIOrKeepRow(req, row)
	if err != nil {
		return nil, err
	}
	if !cat.Valid || strings.TrimSpace(cat.String) == "" {
		cat = sql.NullString{String: "其他", Valid: true}
	}
	updated := *row
	updated.IsDel = 0
	updated.ExpireAt = expireAt
	updated.Category = cat
	updated.SafetyStatus = safety
	updated.PageTitle = title
	updated.PageDescription = desc
	updated.AiSuggestions = sugBytes

	if err := l.svcCtx.Filter.Add([]byte(updated.Surl.String)); err != nil {
		logx.Errorw("Bloom Add failed", logx.LogField{Key: "err", Value: err.Error()})
		return nil, errors.New("短链索引写入失败，请稍后重试")
	}
	if err := l.svcCtx.ShortUrlMapModel.Update(l.ctx, &updated); err != nil {
		logx.Errorw("ShortUrlMapModel.Update failed", logx.LogField{Key: "id", Value: row.Id}, logx.LogField{Key: "err", Value: err.Error()})
		return nil, err
	}
	return l.rowToConvertResponse(&updated, reuseTag), nil
}

// recoverFromDuplicateLurl 兜底处理并发重复提交导致的 unique(lurl) 冲突：
// 当插入阶段命中 duplicate key，回查已存在行并按当前复用策略返回。
func (l *ConverLogic) recoverFromDuplicateLurl(longURL string, insertErr error) (*types.ConvertResponse, bool) {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(insertErr, &mysqlErr) || mysqlErr.Number != 1062 {
		return nil, false
	}
	existing, err := l.svcCtx.ShortUrlMapModel.FindOneByLurl(l.ctx, sql.NullString{String: longURL, Valid: true})
	if err != nil {
		return nil, false
	}
	now := time.Now()
	expired := existing.ExpireAt.Valid && now.After(existing.ExpireAt.Time)
	deleted := existing.IsDel != 0
	switch {
	case !deleted && !expired:
		return l.rowToConvertResponse(existing, linkReuseSameActive), true
	case !deleted && expired:
		return l.rowToConvertResponse(existing, linkReuseRenewedExpired), true
	default:
		return l.rowToConvertResponse(existing, linkReuseReactivatedDeleted), true
	}
}

// mergeAIOrKeepRow 新开转换传 row=nil；续约/复活传入库行。未开 AI 时保留原分类等字段（新开仍为「其他」）。
func (l *ConverLogic) mergeAIOrKeepRow(req *types.ConverRequest, row *model.ShortUrlMap) (
	category sql.NullString,
	safety uint64,
	title sql.NullString,
	desc sql.NullString,
	suggestions []byte,
	err error,
) {
	if !req.EnableAI {
		return l.nonAICategoryFields(row)
	}
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
	if aiErr != nil {
		logx.Errorf("analyze url failed: %v", aiErr)
		return l.nonAICategoryFields(row)
	}
	if analysis == nil {
		return l.nonAICategoryFields(row)
	}
	if analysis.SafetyLevel == "dangerous" {
		return sql.NullString{}, 0, sql.NullString{}, sql.NullString{}, nil, fmt.Errorf("该链接存在安全风险，拒绝生成短链：%s", analysis.SafetyReason)
	}
	category = sql.NullString{}
	if strings.TrimSpace(analysis.Category) != "" {
		category = sql.NullString{String: strings.TrimSpace(analysis.Category), Valid: true}
	}
	safety = getSafetyStatus(analysis.SafetyLevel)
	title = sql.NullString{String: analysis.PageTitle, Valid: analysis.PageTitle != ""}
	desc = sql.NullString{String: analysis.PageDesc, Valid: analysis.PageDesc != ""}
	if len(analysis.Suggestions) > 0 {
		suggestions, _ = json.Marshal(analysis.Suggestions)
	}
	if !category.Valid || strings.TrimSpace(category.String) == "" {
		category = sql.NullString{String: "其他", Valid: true}
	}
	return category, safety, title, desc, suggestions, nil
}

func (l *ConverLogic) nonAICategoryFields(row *model.ShortUrlMap) (
	category sql.NullString,
	safety uint64,
	title sql.NullString,
	desc sql.NullString,
	suggestions []byte,
	err error,
) {
	if row != nil {
		category = row.Category
		if !category.Valid || strings.TrimSpace(category.String) == "" {
			category = sql.NullString{String: "其他", Valid: true}
		}
		return category, row.SafetyStatus, row.PageTitle, row.PageDescription, row.AiSuggestions, nil
	}
	return sql.NullString{String: "其他", Valid: true}, 0, sql.NullString{}, sql.NullString{}, nil, nil
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

// resolveExpireAt 在 RFC3339（ExpireAt）基线上扩展：ExpirePreset、相对时长；优先级：Preset → 相对 → RFC3339。
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
