package presenter_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/presenter"
)

func strPtr(s string) *string { return &s }

func TestToInquiryDetail(t *testing.T) {
	created := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	note := "internal note"
	in := &domain.Inquiry{
		InquiryID:    100,
		Title:        "T",
		Body:         "B",
		ReplyEmail:   "u@example.com",
		Status:       "in_progress",
		InternalNote: &note,
		CreatedAt:    created,
		UpdatedAt:    updated,
	}

	got := presenter.ToInquiryDetail(in)

	assert.Equal(t, int64(100), got.InquiryID)
	assert.Equal(t, "T", got.Title)
	assert.Equal(t, "B", got.Body)
	assert.Equal(t, "u@example.com", got.ReplyEmail)
	assert.Equal(t, "in_progress", got.Status)
	assert.Equal(t, &note, got.InternalNote)
	assert.Equal(t, created, got.CreatedAt)
	assert.Equal(t, updated, got.UpdatedAt)
}

func TestToInquiryDetail_NilNote(t *testing.T) {
	in := &domain.Inquiry{InquiryID: 1, InternalNote: nil}
	got := presenter.ToInquiryDetail(in)
	assert.Nil(t, got.InternalNote, "メモ未設定は wire でも nil で透過する")
}

func TestToInquirySummaries(t *testing.T) {
	created := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	in := []domain.Inquiry{
		{InquiryID: 1, Title: "a", Status: "new", CreatedAt: created, UpdatedAt: created},
		{InquiryID: 2, Title: "b", Status: "closed", CreatedAt: created, UpdatedAt: created, InternalNote: strPtr("xx")},
	}

	got := presenter.ToInquirySummaries(in)

	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].InquiryID)
	assert.Equal(t, "new", got[0].Status)
	assert.Equal(t, int64(2), got[1].InquiryID)
	assert.Equal(t, "closed", got[1].Status)
}

func TestToInquirySummaries_EmptyInputReturnsEmptySlice(t *testing.T) {
	got := presenter.ToInquirySummaries(nil)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
