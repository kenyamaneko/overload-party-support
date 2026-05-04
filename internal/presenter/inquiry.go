package presenter

import (
	"github.com/kenyamaneko/overload-party-support/internal/domain"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// ToInquiryDetail は domain.Inquiry を wire の InquiryDetail に詰め替えます。
func ToInquiryDetail(inq *domain.Inquiry) apisupport.InquiryDetail {
	return apisupport.InquiryDetail{
		InquiryID:    inq.InquiryID,
		Title:        inq.Title,
		Body:         inq.Body,
		ReplyEmail:   inq.ReplyEmail,
		Status:       inq.Status,
		InternalNote: inq.InternalNote,
		CreatedAt:    inq.CreatedAt,
		UpdatedAt:    inq.UpdatedAt,
	}
}

// ToInquirySummary は domain.Inquiry を wire の InquirySummary に詰め替えます。
func ToInquirySummary(inq domain.Inquiry) apisupport.InquirySummary {
	return apisupport.InquirySummary{
		InquiryID:  inq.InquiryID,
		Title:      inq.Title,
		ReplyEmail: inq.ReplyEmail,
		Status:     inq.Status,
		CreatedAt:  inq.CreatedAt,
		UpdatedAt:  inq.UpdatedAt,
	}
}

// ToInquirySummaries は domain.Inquiry slice を wire の InquirySummary slice に詰め替えます。
func ToInquirySummaries(items []domain.Inquiry) []apisupport.InquirySummary {
	out := make([]apisupport.InquirySummary, 0, len(items))
	for _, inq := range items {
		out = append(out, ToInquirySummary(inq))
	}
	return out
}
