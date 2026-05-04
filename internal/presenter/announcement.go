package presenter

import (
	"github.com/kenyamaneko/overload-party-support/internal/domain"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// ToAnnouncementSummary は domain.AnnouncementSummary を wire の AnnouncementSummary に詰め替えます。
func ToAnnouncementSummary(s domain.AnnouncementSummary) apisupport.AnnouncementSummary {
	return apisupport.AnnouncementSummary{
		AnnouncementID: s.AnnouncementID,
		Type:           s.Type,
		Title:          s.Title,
		PublishedAt:    s.PublishedAt,
	}
}

// ToAnnouncementSummaries は domain.AnnouncementSummary slice を wire slice に詰め替えます。
func ToAnnouncementSummaries(items []domain.AnnouncementSummary) []apisupport.AnnouncementSummary {
	out := make([]apisupport.AnnouncementSummary, 0, len(items))
	for _, it := range items {
		out = append(out, ToAnnouncementSummary(it))
	}
	return out
}

// ToAnnouncementDetail は domain.AnnouncementDetail を wire の AnnouncementDetail に詰め替えます。
func ToAnnouncementDetail(d *domain.AnnouncementDetail) apisupport.AnnouncementDetail {
	return apisupport.AnnouncementDetail{
		AnnouncementID: d.AnnouncementID,
		Type:           d.Type,
		Title:          d.Title,
		Body:           d.Body,
		PublishedAt:    d.PublishedAt,
	}
}
