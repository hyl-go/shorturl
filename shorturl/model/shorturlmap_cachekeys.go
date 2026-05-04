package model

import (
	"database/sql"
	"fmt"
)

// 以下前缀须与 shorturlmapmodel_gen.go 中 cacheShorturlShortUrlMap*Prefix 保持一致（goctl 重生成后请人工核对）。
const (
	cacheShortURLMapIDPrefix   = "cache:shorturl:shortUrlMap:id:"
	cacheShortURLMapLurlPrefix = "cache:shorturl:shortUrlMap:lurl:"
	cacheShortURLMapMd5Prefix  = "cache:shorturl:shortUrlMap:md5:"
	cacheShortURLMapSurlPrefix = "cache:shorturl:shortUrlMap:surl:"
)

// ShortURLMapRowCacheKeys 生成该行在 CachedConn 下使用的索引缓存键，供 GC 等旁路删除后同步失效。
func ShortURLMapRowCacheKeys(id uint64, lurl, md5, surl sql.NullString) []string {
	return []string{
		fmt.Sprintf("%s%v", cacheShortURLMapIDPrefix, id),
		fmt.Sprintf("%s%v", cacheShortURLMapLurlPrefix, lurl),
		fmt.Sprintf("%s%v", cacheShortURLMapMd5Prefix, md5),
		fmt.Sprintf("%s%v", cacheShortURLMapSurlPrefix, surl),
	}
}
