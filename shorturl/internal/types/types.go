package types

type ConverRequest struct {
	LongUrl          string `json:"longURL" validate:"required"`
	CustomShortURL   string `json:"customShortURL,optional"`
	ExpirePreset     string `json:"expirePreset,optional"`     // none|never|30m|1h|1d|7d
	ExpireAfterValue int64  `json:"expireAfterValue,optional"` // 与 ExpireAfterUnit 同时使用
	ExpireAfterUnit  string `json:"expireAfterUnit,optional"`  // minute hour day week month year
	ExpireAt         string `json:"expireAt,optional"`         // RFC3339，兼容旧客户端
	EnableAI         bool   `json:"enableAI,optional"`
}

type ConvertResponse struct {
	ShortUrl      string   `json:"shortURL"`
	ExpireAt      string   `json:"expireAt"`
	Category      string   `json:"category,omitempty"`
	SafetyStatus  string   `json:"safetyStatus,omitempty"`
	AiSuggestions []string `json:"aiSuggestions,omitempty"`
	// linkReuse：same_active 幂等命中有效链 | renewed_expired 过期后同记录续约 | reactivated_deleted 软删复活 | inserted_new 新插入
	LinkReuse string `json:"linkReuse,omitempty"`
}

type ShowRequest struct {
	ShortURL  string `path:"shortURL" validate:"required"`
	IP        string `json:"-"`
	Country   string `json:"-"`
	Region    string `json:"-"`
	UserAgent string `json:"-"`
	Referer   string `json:"-"`
}

type ShowResponse struct {
	LongURL string `json:"longURL"`
}

type StatsRequest struct {
	ShortURL  string `form:"shortURL" validate:"required"`
	StartDate string `form:"startDate" validate:"required"`
	EndDate   string `form:"endDate" validate:"required"`
}

type ChartPoint struct {
	Date string `json:"date"`
	PV   int64  `json:"pv"`
	UV   int64  `json:"uv"`
}

type DeviceStats struct {
	MobileRate float64       `json:"mobileRate"`
	Breakdown  []DeviceCount `json:"breakdown"`
}

type GeoStat struct {
	Country string `json:"country"`
	City    string `json:"city"`
	Count   int64  `json:"count"`
}

type GeoAgg struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type StatsResponse struct {
	TotalPV      int64        `json:"totalPV"`
	TotalUV      int64        `json:"totalUV"`
	ChartData    []ChartPoint `json:"chartData"`
	DeviceStats  DeviceStats  `json:"deviceStats"`
	GeoStats     []GeoStat    `json:"geoStats"`
	GeoByCountry []GeoAgg     `json:"geoByCountry"`
	GeoByRegion  []GeoAgg     `json:"geoByRegion"`
}

type DeviceCount struct {
	Device string `json:"device"`
	Count  int64  `json:"count"`
}

type AnalyzeRequest struct {
	ShortURL  string `json:"shortURL" validate:"required"`
	StartDate string `json:"startDate" validate:"required"`
	EndDate   string `json:"endDate" validate:"required"`
}

type AIReport struct {
	Title       string   `json:"title,omitempty"`
	Summary     string   `json:"summary"`
	Trends      []string `json:"trends"`
	Anomalies   []string `json:"anomalies"`
	Suggestions []string `json:"suggestions"`
	Markdown    string   `json:"markdown,omitempty"`
}

type ReportJobInfo struct {
	JobId  string `json:"jobId"`
	Status string `json:"status"`
}

type AnalyzeResponse struct {
	Statistics StatsResponse `json:"statistics"`
	AIReport   *AIReport     `json:"aiReport,omitempty"`
	ReportJob  ReportJobInfo `json:"reportJob"`
}

type AnalyzeReportStatusRequest struct {
	JobId string `form:"jobId" validate:"required"`
}

type AnalyzeReportStatusResponse struct {
	Status          string    `json:"status"`
	AIReport        *AIReport `json:"aiReport,omitempty"`
	Error           string    `json:"error,omitempty"`
	MarkdownEdited  string    `json:"markdownEdited,omitempty"`
}

type AnalyzeReportUpdateRequest struct {
	JobId    string `json:"jobId" validate:"required"`
	Markdown string `json:"markdown" validate:"required"`
}

type AnalyzeReportUpdateResponse struct {
	Ok bool `json:"ok"`
}

type LinkListRequest struct {
	Page     int64  `form:"page,optional"`
	PageSize int64  `form:"pageSize,optional"`
	Category string `form:"category,optional"`
}

type LinkListItem struct {
	Id              uint64   `json:"id"`
	LongURL         string   `json:"longURL"`
	ShortURL        string   `json:"shortURL"`
	ShortPath       string   `json:"shortPath"`
	Category        string   `json:"category,omitempty"`
	SafetyStatus    string   `json:"safetyStatus,omitempty"`
	ExpireAt        string   `json:"expireAt,omitempty"`
	CreateAt        string   `json:"createAt"`
	UpdateAt        string   `json:"updateAt,omitempty"`
	PageTitle       string   `json:"pageTitle,omitempty"`
	PageDescription string   `json:"pageDescription,omitempty"`
	AiSuggestions   []string `json:"aiSuggestions,omitempty"`
	Md5             string   `json:"md5,omitempty"`
}

type LinkUpdateRequest struct {
	Id               uint64 `path:"id" validate:"required,gt=0"`
	LongURL          string `json:"longURL,optional"`
	Category         string `json:"category,optional"`
	NoExpire         bool   `json:"noExpire,optional"`
	ExpirePreset     string `json:"expirePreset,optional"`
	ExpireAfterValue int64  `json:"expireAfterValue,optional"`
	ExpireAfterUnit  string `json:"expireAfterUnit,optional"`
	ExpireAt         string `json:"expireAt,optional"`
}

type LinkUpdateResponse struct {
	Item LinkListItem `json:"item"`
}

type LinkDeleteRequest struct {
	Id uint64 `path:"id" validate:"required,gt=0"`
}

type LinkDeleteResponse struct {
	Ok bool `json:"ok"`
}

type LinkListResponse struct {
	Total int64          `json:"total"`
	List  []LinkListItem `json:"list"`
}

type LinkCategoriesResponse struct {
	Categories []string `json:"categories"`
}

// PerformanceSnapshotResponse 管理端性能面板：主机 + MySQL + Redis 快照（单次采集，非长期时序）。
type PerformanceSnapshotResponse struct {
	CollectedAt string     `json:"collectedAt"`
	Host        PerfHost   `json:"host"`
	CPU         PerfCPU    `json:"cpu"`
	Memory      PerfMemory `json:"memory"`
	Disk        PerfDisk   `json:"disk"`
	DiskIO      PerfDiskIO `json:"diskIO"`
	MySQL       PerfMySQL  `json:"mysql"`
	Redis       PerfRedis  `json:"redis"`
}

type PerfHost struct {
	Hostname   string `json:"hostname"`
	OS         string `json:"os"`
	Platform   string `json:"platform"`
	Kernel     string `json:"kernel"`
	UptimeSec  uint64 `json:"uptimeSec"`
	Procs      uint64 `json:"procs"`
	BootTime   uint64 `json:"bootTime"`
	GoRoutines int64  `json:"goRoutines"`
}

type PerfCPU struct {
	UsagePercent float64 `json:"usagePercent"`
	Load1        float64 `json:"load1,omitempty"`
	Load5        float64 `json:"load5,omitempty"`
	Load15       float64 `json:"load15,omitempty"`
}

type PerfMemory struct {
	TotalBytes     int64   `json:"totalBytes"`
	AvailableBytes int64   `json:"availableBytes"`
	UsedBytes      int64   `json:"usedBytes"`
	UsedPercent    float64 `json:"usedPercent"`
}

type PerfDisk struct {
	Path              string  `json:"path"`
	TotalBytes        int64   `json:"totalBytes"`
	UsedBytes         int64   `json:"usedBytes"`
	FreeBytes         int64   `json:"freeBytes"`
	UsedPercent       float64 `json:"usedPercent"`
	InodesTotal       int64   `json:"inodesTotal"`
	InodesUsed        int64   `json:"inodesUsed"`
	InodesFree        int64   `json:"inodesFree"`
	InodesUsedPercent float64 `json:"inodesUsedPercent"`
}

type PerfDiskIO struct {
	ReadBytes  uint64 `json:"readBytes"`
	WriteBytes uint64 `json:"writeBytes"`
	ReadCount  uint64 `json:"readCount"`
	WriteCount uint64 `json:"writeCount"`
	Note       string `json:"note,omitempty"`
}

type PerfMySQL struct {
	Ok                 bool    `json:"ok"`
	Error              string  `json:"error,omitempty"`
	PingMs             float64 `json:"pingMs"`
	Version            string  `json:"version,omitempty"`
	MaxConnections     int64   `json:"maxConnections"`
	ThreadsConnected   int64   `json:"threadsConnected"`
	ThreadsRunning     int64   `json:"threadsRunning"`
	Questions          int64   `json:"questions"`
	SlowQueries        int64   `json:"slowQueries"`
	UptimeSec          int64   `json:"uptimeSec"`
	MaxUsedConnections int64   `json:"maxUsedConnections"`
}

type PerfRedis struct {
	Ok                     bool    `json:"ok"`
	Error                  string  `json:"error,omitempty"`
	PingMs                 float64 `json:"pingMs"`
	RedisVersion           string  `json:"redisVersion,omitempty"`
	UsedMemory             int64   `json:"usedMemory"`
	UsedMemoryHuman        string  `json:"usedMemoryHuman,omitempty"`
	ConnectedClients       int64   `json:"connectedClients"`
	TotalCommandsProcessed int64   `json:"totalCommandsProcessed"`
	InstantaneousOpsPerSec int64   `json:"instantaneousOpsPerSec"`
	KeyspaceHits           int64   `json:"keyspaceHits"`
	KeyspaceMisses         int64   `json:"keyspaceMisses"`
	RdbLastSaveTime        int64   `json:"rdbLastSaveTime"`
	AofEnabled             string  `json:"aofEnabled,omitempty"`
}
