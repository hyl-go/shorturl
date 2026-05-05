package logic

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"shorturl/internal/svc"
	"shorturl/internal/types"
)

type StatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StatsLogic {
	return &StatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Stats 以 access_log 为**唯一事实源**，保证总 PV、按日 PV、设备/地域「访问次数」同一套区间、同一套 COUNT 口径，互相对得齐。
// access_stats 仍由 Asynq 按小时异步维护，用于离线/扩展/降载，**不直接参与本接口**（避免汇总表与明细因任务延迟、缺桶产生偏差）。
func (l *StatsLogic) Stats(req *types.StatsRequest) (*types.StatsResponse, error) {
	start, err := time.ParseInLocation("2006-01-02", req.StartDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("startDate 格式错误，应为 YYYY-MM-DD")
	}
	end, err := time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("endDate 格式错误，应为 YYYY-MM-DD")
	}
	endExclusive := end.AddDate(0, 0, 1)
	return l.statsFromAccessLog(req, start, end, endExclusive)
}

// statsFromAccessLog 全量从 access_log 聚合，与展示「访问次数」严格一致。
func (l *StatsLogic) statsFromAccessLog(req *types.StatsRequest, start, end, endExclusive time.Time) (*types.StatsResponse, error) {
	type totalRow struct {
		Pv int64 `db:"pv"`
		Uv int64 `db:"uv"`
	}
	var totals totalRow
	totalQuery := `SELECT COALESCE(COUNT(*), 0) AS pv,
COALESCE(COUNT(DISTINCT NULLIF(TRIM(ip), '')), 0) AS uv
FROM access_log
WHERE surl = ? AND access_time >= ? AND access_time < ?`
	if err := l.svcCtx.DbConn.QueryRowCtx(l.ctx, &totals, totalQuery, req.ShortURL, start, endExclusive); err != nil {
		return nil, err
	}

	type dayRow struct {
		Date string `db:"date"`
		Pv   int64  `db:"pv"`
		Uv   int64  `db:"uv"`
	}
	var dayRows []dayRow
	dayQuery := `SELECT DATE_FORMAT(access_time, '%Y-%m-%d') AS date,
COALESCE(COUNT(*), 0) AS pv,
COALESCE(COUNT(DISTINCT NULLIF(TRIM(ip), '')), 0) AS uv
FROM access_log
WHERE surl = ? AND access_time >= ? AND access_time < ?
GROUP BY DATE_FORMAT(access_time, '%Y-%m-%d')
ORDER BY date ASC`
	if err := l.svcCtx.DbConn.QueryRowsCtx(l.ctx, &dayRows, dayQuery, req.ShortURL, start, endExclusive); err != nil {
		return nil, err
	}

	byDay := make(map[string]struct{ Pv, Uv int64 }, len(dayRows))
	for _, row := range dayRows {
		byDay[row.Date] = struct{ Pv, Uv int64 }{Pv: row.Pv, Uv: row.Uv}
	}

	resp := &types.StatsResponse{
		TotalPV:      totals.Pv,
		TotalUV:      totals.Uv,
		ChartData:    make([]types.ChartPoint, 0),
		GeoStats:     make([]types.GeoStat, 0),
		GeoByCountry: make([]types.GeoAgg, 0),
		GeoByRegion:  make([]types.GeoAgg, 0),
	}
	for t := start; !t.After(end); t = t.AddDate(0, 0, 1) {
		ds := t.Format("2006-01-02")
		pt := byDay[ds]
		resp.ChartData = append(resp.ChartData, types.ChartPoint{
			Date: ds,
			PV:   pt.Pv,
			UV:   pt.Uv,
		})
	}

	type sumRow struct {
		Mobile int64 `db:"mobile_cnt"`
		Total  int64 `db:"total_cnt"`
	}
	var sr sumRow
	deviceQuery := `SELECT
COALESCE(SUM(CASE WHEN device_type = 'mobile' THEN 1 ELSE 0 END), 0) AS mobile_cnt,
COALESCE(COUNT(*), 0) AS total_cnt
FROM access_log
WHERE surl = ? AND access_time >= ? AND access_time < ?`
	if err := l.svcCtx.DbConn.QueryRowCtx(l.ctx, &sr, deviceQuery, req.ShortURL, start, endExclusive); err != nil {
		return nil, err
	}
	if sr.Total > 0 {
		resp.DeviceStats.MobileRate = float64(sr.Mobile) * 100 / float64(sr.Total)
	}

	type deviceRow struct {
		DeviceType sql.NullString `db:"device_type"`
		Count      int64          `db:"count"`
	}
	var deviceRows []deviceRow
	deviceBreakdownQuery := `SELECT device_type, COALESCE(COUNT(1), 0) AS count
FROM access_log
WHERE surl = ? AND access_time >= ? AND access_time < ?
GROUP BY device_type
ORDER BY count DESC`
	if err := l.svcCtx.DbConn.QueryRowsCtx(l.ctx, &deviceRows, deviceBreakdownQuery, req.ShortURL, start, endExclusive); err != nil {
		return nil, err
	}
	resp.DeviceStats.Breakdown = make([]types.DeviceCount, 0, len(deviceRows))
	for _, item := range deviceRows {
		resp.DeviceStats.Breakdown = append(resp.DeviceStats.Breakdown, types.DeviceCount{
			Device: geoDimDisplay(item.DeviceType, "unknown"),
			Count:  item.Count,
		})
	}

	type geoRow struct {
		Country sql.NullString `db:"country"`
		City    sql.NullString `db:"city"`
		Count   int64          `db:"count"`
	}
	var geoRows []geoRow
	geoQuery := `SELECT country, city, COALESCE(COUNT(1), 0) AS count
FROM access_log
WHERE surl = ? AND access_time >= ? AND access_time < ?
GROUP BY country, city
ORDER BY count DESC
LIMIT 5`
	if err := l.svcCtx.DbConn.QueryRowsCtx(l.ctx, &geoRows, geoQuery, req.ShortURL, start, endExclusive); err != nil {
		return nil, err
	}
	for _, item := range geoRows {
		resp.GeoStats = append(resp.GeoStats, types.GeoStat{
			Country: geoDimDisplay(item.Country, "未知"),
			City:    geoDimDisplay(item.City, "未知"),
			Count:   item.Count,
		})
	}

	type geoAggRow struct {
		Name  sql.NullString `db:"name"`
		Count int64          `db:"count"`
	}
	var countryRows []geoAggRow
	countryQuery := `SELECT country AS name, COALESCE(COUNT(1), 0) AS count
FROM access_log
WHERE surl = ? AND access_time >= ? AND access_time < ?
GROUP BY country
ORDER BY count DESC
LIMIT 10`
	if err := l.svcCtx.DbConn.QueryRowsCtx(l.ctx, &countryRows, countryQuery, req.ShortURL, start, endExclusive); err != nil {
		return nil, err
	}
	for _, item := range countryRows {
		resp.GeoByCountry = append(resp.GeoByCountry, types.GeoAgg{
			Name:  geoDimDisplay(item.Name, "未知国家"),
			Count: item.Count,
		})
	}

	var regionRows []geoAggRow
	regionQuery := `SELECT city AS name, COALESCE(COUNT(1), 0) AS count
FROM access_log
WHERE surl = ? AND access_time >= ? AND access_time < ?
GROUP BY city
ORDER BY count DESC
LIMIT 10`
	if err := l.svcCtx.DbConn.QueryRowsCtx(l.ctx, &regionRows, regionQuery, req.ShortURL, start, endExclusive); err != nil {
		return nil, err
	}
	for _, item := range regionRows {
		resp.GeoByRegion = append(resp.GeoByRegion, types.GeoAgg{
			Name:  geoDimDisplay(item.Name, "未知地区"),
			Count: item.Count,
		})
	}

	return resp, nil
}

func geoDimDisplay(ns sql.NullString, emptyLabel string) string {
	if ns.Valid {
		s := strings.TrimSpace(ns.String)
		if s != "" {
			return s
		}
	}
	return emptyLabel
}
