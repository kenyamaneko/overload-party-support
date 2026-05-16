package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// 仕様 (FEATURE_SPEC §6.3): DeriveState は (PublishedAt, ExpiresAt) と now から state を一意に導出する。
// 判定順序が排他性を担保している。特に PublishedAt IS NULL の record は ExpiresAt の値に依存せず Draft。
func TestDeriveState(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name        string
		publishedAt *time.Time
		expiresAt   *time.Time
		want        string
	}{
		{
			name:        "PublishedAt=NULL → Draft (ExpiresAt 未設定)",
			publishedAt: nil,
			expiresAt:   nil,
			want:        domain.StateDraft,
		},
		{
			name:        "PublishedAt=NULL → Draft (ExpiresAt 過去でも Expired にならない)",
			publishedAt: nil,
			expiresAt:   &past,
			want:        domain.StateDraft,
		},
		{
			name:        "PublishedAt>now → Scheduled",
			publishedAt: &future,
			expiresAt:   nil,
			want:        domain.StateScheduled,
		},
		{
			name:        "PublishedAt<=now ∧ ExpiresAt=NULL → Published",
			publishedAt: &past,
			expiresAt:   nil,
			want:        domain.StatePublished,
		},
		{
			name:        "PublishedAt<=now ∧ ExpiresAt>now → Published",
			publishedAt: &past,
			expiresAt:   &future,
			want:        domain.StatePublished,
		},
		{
			name:        "PublishedAt<=now ∧ ExpiresAt<=now → Expired",
			publishedAt: &past,
			expiresAt:   &past,
			want:        domain.StateExpired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.DeriveState(domain.Announcement{
				PublishedAt: tc.publishedAt,
				ExpiresAt:   tc.expiresAt,
			}, now)
			assert.Equal(t, tc.want, got)
		})
	}
}
