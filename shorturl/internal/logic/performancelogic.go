package logic

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"time"

	"shorturl/internal/svc"
	"shorturl/internal/types"

	"github.com/redis/go-redis/v9"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/zeromicro/go-zero/core/logx"
)

type PerformanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPerformanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PerformanceLogic {
	return &PerformanceLogic{ctx: ctx, svcCtx: svcCtx}
}

// PerformanceSnapshot 聚合主机与依赖（MySQL / Redis）的轻量指标，供管理端性能面板使用。
func (l *PerformanceLogic) PerformanceSnapshot() (*types.PerformanceSnapshotResponse, error) {
	out := &types.PerformanceSnapshotResponse{
		CollectedAt: time.Now().Format(time.RFC3339),
	}

	if hi, err := host.InfoWithContext(l.ctx); err == nil {
		out.Host = types.PerfHost{
			Hostname:   hi.Hostname,
			OS:         hi.OS,
			Platform:   hi.Platform,
			Kernel:     hi.KernelVersion,
			UptimeSec:  hi.Uptime,
			Procs:      hi.Procs,
			BootTime:   hi.BootTime,
			GoRoutines: int64(runtime.NumGoroutine()),
		}
	} else {
		logx.Errorf("performance host info: %v", err)
	}

	if pct, err := cpu.PercentWithContext(l.ctx, 200*time.Millisecond, false); err == nil && len(pct) > 0 {
		out.CPU.UsagePercent = pct[0]
	} else if err != nil {
		logx.Errorf("performance cpu: %v", err)
	}
	if la, err := load.AvgWithContext(l.ctx); err == nil {
		out.CPU.Load1 = la.Load1
		out.CPU.Load5 = la.Load5
		out.CPU.Load15 = la.Load15
	}

	if vm, err := mem.VirtualMemoryWithContext(l.ctx); err == nil {
		out.Memory = types.PerfMemory{
			TotalBytes:     int64(vm.Total),
			AvailableBytes: int64(vm.Available),
			UsedBytes:      int64(vm.Used),
			UsedPercent:    vm.UsedPercent,
		}
	} else {
		logx.Errorf("performance mem: %v", err)
	}

	root := "/"
	if runtime.GOOS == "windows" {
		root = `C:\`
	}
	if du, err := disk.UsageWithContext(l.ctx, root); err == nil {
		out.Disk = types.PerfDisk{
			Path:              du.Path,
			TotalBytes:        int64(du.Total),
			UsedBytes:         int64(du.Used),
			FreeBytes:         int64(du.Free),
			UsedPercent:       du.UsedPercent,
			InodesTotal:       int64(du.InodesTotal),
			InodesUsed:        int64(du.InodesUsed),
			InodesFree:        int64(du.InodesFree),
			InodesUsedPercent: du.InodesUsedPercent,
		}
	} else {
		logx.Errorf("performance disk usage: %v", err)
	}

	if counters, err := disk.IOCountersWithContext(l.ctx); err == nil {
		var rb, wb, rc, wc uint64
		for _, c := range counters {
			rb += c.ReadBytes
			wb += c.WriteBytes
			rc += c.ReadCount
			wc += c.WriteCount
		}
		out.DiskIO = types.PerfDiskIO{
			ReadBytes:  rb,
			WriteBytes: wb,
			ReadCount:  rc,
			WriteCount: wc,
			Note:       "自系统启动以来的累计值（非速率）",
		}
	} else {
		logx.Errorf("performance disk io: %v", err)
	}

	l.fillMySQL(out)
	l.fillRedis(out)

	return out, nil
}

type mysqlStatusRow struct {
	Name  string `db:"Variable_name"`
	Value string `db:"Value"`
}

func (l *PerformanceLogic) fillMySQL(out *types.PerformanceSnapshotResponse) {
	m := &out.MySQL
	t0 := time.Now()
	if err := l.svcCtx.DbConn.QueryRowCtx(l.ctx, &m.Version, "SELECT VERSION()"); err != nil {
		m.Ok = false
		m.Error = err.Error()
		return
	}
	m.PingMs = time.Since(t0).Seconds() * 1000
	if err := l.svcCtx.DbConn.QueryRowCtx(l.ctx, &m.MaxConnections, "SELECT @@max_connections"); err != nil {
		m.Ok = false
		m.Error = err.Error()
		return
	}
	var rows []mysqlStatusRow
	q := `SHOW GLOBAL STATUS WHERE Variable_name IN (
'Threads_connected','Threads_running','Questions','Slow_queries','Uptime','Max_used_connections')`
	if err := l.svcCtx.DbConn.QueryRowsCtx(l.ctx, &rows, q); err != nil {
		m.Ok = false
		m.Error = err.Error()
		return
	}
	for _, r := range rows {
		v, convErr := strconv.ParseInt(r.Value, 10, 64)
		if convErr != nil {
			continue
		}
		switch r.Name {
		case "Threads_connected":
			m.ThreadsConnected = v
		case "Threads_running":
			m.ThreadsRunning = v
		case "Questions":
			m.Questions = v
		case "Slow_queries":
			m.SlowQueries = v
		case "Uptime":
			m.UptimeSec = v
		case "Max_used_connections":
			m.MaxUsedConnections = v
		}
	}
	m.Ok = true
}

func (l *PerformanceLogic) fillRedis(out *types.PerformanceSnapshotResponse) {
	r := &out.Redis
	if len(l.svcCtx.Config.CacheRedis) == 0 {
		r.Error = "未配置 CacheRedis"
		return
	}
	node := l.svcCtx.Config.CacheRedis[0].RedisConf
	if strings.TrimSpace(node.Host) == "" {
		r.Error = "Redis Host 为空"
		return
	}
	cli := redis.NewClient(&redis.Options{
		Addr:     node.Host,
		Password: node.Pass,
		DB:       0,
	})
	defer func() { _ = cli.Close() }()

	t0 := time.Now()
	if err := cli.Ping(l.ctx).Err(); err != nil {
		r.Ok = false
		r.Error = err.Error()
		return
	}
	r.PingMs = time.Since(t0).Seconds() * 1000

	info, err := cli.Info(l.ctx, "server", "memory", "stats", "clients", "persistence").Result()
	if err != nil {
		r.Ok = false
		r.Error = err.Error()
		return
	}
	m := parseRedisInfo(info)
	r.RedisVersion = m["redis_version"]
	r.UsedMemory = parseInt64(m["used_memory"])
	r.UsedMemoryHuman = m["used_memory_human"]
	r.ConnectedClients = parseInt64(m["connected_clients"])
	r.TotalCommandsProcessed = parseInt64(m["total_commands_processed"])
	r.InstantaneousOpsPerSec = parseInt64(m["instantaneous_ops_per_sec"])
	r.KeyspaceHits = parseInt64(m["keyspace_hits"])
	r.KeyspaceMisses = parseInt64(m["keyspace_misses"])
	r.RdbLastSaveTime = parseInt64(m["rdb_last_save_time"])
	r.AofEnabled = m["aof_enabled"]
	r.Ok = true
}

func parseRedisInfo(s string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		out[k] = v
	}
	return out
}

func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
