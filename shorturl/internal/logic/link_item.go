package logic

import (
	"encoding/json"

	"shorturl/internal/types"
	"shorturl/model"
)

func shortURLRowToListItem(row model.ShortUrlMap, domain string) types.LinkListItem {
	item := types.LinkListItem{
		Id:           row.Id,
		LongURL:      row.Lurl.String,
		ShortPath:    row.Surl.String,
		ShortURL:     domain + "/" + row.Surl.String,
		Category:     normalizeCategoryDisplay(row.Category.String),
		SafetyStatus: getSafetyLevelString(row.SafetyStatus),
		CreateAt:     formatLocalDateTime(row.CreateAt),
	}
	if row.Md5.Valid {
		item.Md5 = row.Md5.String
	}
	if row.PageTitle.Valid {
		item.PageTitle = row.PageTitle.String
	}
	if row.PageDescription.Valid {
		item.PageDescription = row.PageDescription.String
	}
	if len(row.AiSuggestions) > 0 {
		var sug []string
		if err := json.Unmarshal(row.AiSuggestions, &sug); err == nil {
			item.AiSuggestions = sug
		}
	}
	if row.UpdateAt.Valid {
		item.UpdateAt = formatLocalDateTime(row.UpdateAt.Time)
	}
	if row.ExpireAt.Valid {
		item.ExpireAt = formatLocalDateTime(row.ExpireAt.Time)
	}
	return item
}
