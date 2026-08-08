package domain

import "time"

// AnnouncementSummary は ListPublished の戻り値要素 (本体 + lang 固有の title)。
// wire の apisupport.AnnouncementSummary とは別型で、presenter が詰め替える。
type AnnouncementSummary struct {
	AnnouncementID int64
	Type           string
	Title          string
	PublishedAt    time.Time
}

// AnnouncementDetail は GetPublishedDetail の戻り値 (本体 + lang 固有の title/body)。
// wire の apisupport.AnnouncementDetail とは別型で、presenter が詰め替える。
type AnnouncementDetail struct {
	AnnouncementID int64
	Type           string
	Title          string
	Body           string
	PublishedAt    *time.Time
}
