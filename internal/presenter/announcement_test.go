package presenter_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/presenter"
)

func TestToAnnouncementSummary(t *testing.T) {
	pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	in := domain.AnnouncementSummary{
		AnnouncementID: 42,
		Type:           "info",
		Title:          "T",
		PublishedAt:    pub,
	}

	got := presenter.ToAnnouncementSummary(in)

	assert.Equal(t, int64(42), got.AnnouncementID)
	assert.Equal(t, "info", got.Type)
	assert.Equal(t, "T", got.Title)
	assert.Equal(t, pub, got.PublishedAt)
}

func TestToAnnouncementSummaries(t *testing.T) {
	pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	in := []domain.AnnouncementSummary{
		{AnnouncementID: 1, Title: "a", PublishedAt: pub},
		{AnnouncementID: 2, Title: "b", PublishedAt: pub},
	}

	got := presenter.ToAnnouncementSummaries(in)

	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].AnnouncementID)
	assert.Equal(t, int64(2), got[1].AnnouncementID)
}

func TestToAnnouncementSummaries_EmptyInputReturnsEmptySlice(t *testing.T) {
	got := presenter.ToAnnouncementSummaries(nil)
	assert.NotNil(t, got, "JSON エンコード時に null ではなく [] にするため空 slice を返す")
	assert.Empty(t, got)
}

func TestToAnnouncementDetail(t *testing.T) {
	pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	in := &domain.AnnouncementDetail{
		AnnouncementID: 7,
		Type:           "maintenance",
		Title:          "T",
		Body:           "B",
		PublishedAt:    &pub,
	}

	got := presenter.ToAnnouncementDetail(in)

	assert.Equal(t, int64(7), got.AnnouncementID)
	assert.Equal(t, "maintenance", got.Type)
	assert.Equal(t, "T", got.Title)
	assert.Equal(t, "B", got.Body)
	assert.Equal(t, &pub, got.PublishedAt)
}

func TestToAnnouncementDetail_NilPublishedAt(t *testing.T) {
	in := &domain.AnnouncementDetail{
		AnnouncementID: 8,
		Type:           "info",
		Title:          "T",
		Body:           "B",
		PublishedAt:    nil,
	}

	got := presenter.ToAnnouncementDetail(in)

	assert.Nil(t, got.PublishedAt, "下書きの published_at=null は wire でも nil で透過する")
}
