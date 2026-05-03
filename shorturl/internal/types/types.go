package types

type ConverRequest struct {
	LongUrl          string `json:"longURL" validate:"required"`
	CustomShortURL   string `json:"customShortURL,optional"`
	ExpirePreset     string `json:"expirePreset,optional"`      // none|never|30m|1h|1d|7d
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
}

type ShowRequest struct {
	ShortURL  string `path:"shortURL" validate:"required"`
	IP        string `json:"-"`
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
	MobileRate float64 `json:"mobileRate"`
}

type GeoStat struct {
	Country string `json:"country"`
	City    string `json:"city"`
	Count   int64  `json:"count"`
}

type StatsResponse struct {
	TotalPV     int64        `json:"totalPV"`
	TotalUV     int64        `json:"totalUV"`
	ChartData   []ChartPoint `json:"chartData"`
	DeviceStats DeviceStats  `json:"deviceStats"`
	GeoStats    []GeoStat    `json:"geoStats"`
}

type AnalyzeRequest struct {
	ShortURL  string `json:"shortURL" validate:"required"`
	StartDate string `json:"startDate" validate:"required"`
	EndDate   string `json:"endDate" validate:"required"`
}

type AIReport struct {
	Summary     string   `json:"summary"`
	Trends      []string `json:"trends"`
	Anomalies   []string `json:"anomalies"`
	Suggestions []string `json:"suggestions"`
}

type AnalyzeResponse struct {
	Statistics StatsResponse `json:"statistics"`
	AIReport   AIReport      `json:"aiReport"`
}

type LinkListRequest struct {
	Page       int64  `form:"page,optional"`
	PageSize   int64  `form:"pageSize,optional"`
	Category   string `form:"category,optional"`
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
