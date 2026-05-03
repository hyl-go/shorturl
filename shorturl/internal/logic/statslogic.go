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
	dayRows := make([]dayRow, 0)
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

	resp := &types.StatsResponse{
		TotalPV:   totals.Pv,
		TotalUV:   totals.Uv,
		ChartData: make([]types.ChartPoint, 0, len(dayRows)),
		GeoStats:  make([]types.GeoStat, 0),
	}
	for _, row := range dayRows {
		resp.ChartData = append(resp.ChartData, types.ChartPoint{
			Date: row.Date,
			PV:   row.Pv,
			UV:   row.Uv,
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

	type geoRow struct {
		Country sql.NullString `db:"country"`
		City    sql.NullString `db:"city"`
		Count   int64          `db:"count"`
	}
	geoRows := make([]geoRow, 0)
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
