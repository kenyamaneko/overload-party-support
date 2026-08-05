package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

func TestDeriveState(t *testing.T) {
	// 判定順序が state の排他性を担保する。特に PublishedAt が NULL の record は ExpiresAt の値に依存せず Draft になる。
	t.Run("公開状態の導出", func(t *testing.T) {
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
				name:        "PublishedAtがNULLでExpiresAt未設定のとき、Draftになる",
				publishedAt: nil,
				expiresAt:   nil,
				want:        domain.StateDraft,
			},
			{
				name:        "PublishedAtがNULLでExpiresAtが過去のとき、ExpiredにならずDraftになる",
				publishedAt: nil,
				expiresAt:   &past,
				want:        domain.StateDraft,
			},
			{
				name:        "PublishedAtがnowより未来のとき、Scheduledになる",
				publishedAt: &future,
				expiresAt:   nil,
				want:        domain.StateScheduled,
			},
			{
				name:        "PublishedAt<=nowでExpiresAtがNULLのとき、Publishedになる",
				publishedAt: &past,
				expiresAt:   nil,
				want:        domain.StatePublished,
			},
			{
				name:        "PublishedAt<=nowでExpiresAtが未来のとき、Publishedになる",
				publishedAt: &past,
				expiresAt:   &future,
				want:        domain.StatePublished,
			},
			{
				name:        "PublishedAt<=nowでExpiresAtが過去のとき、Expiredになる",
				publishedAt: &past,
				expiresAt:   &past,
				want:        domain.StateExpired,
			},
			{
				name:        "公開時刻が現在時刻と等しいとき、予約公開にならず公開中になる",
				publishedAt: &now,
				expiresAt:   nil,
				want:        domain.StatePublished,
			},
			{
				name:        "失効時刻が現在時刻と等しいとき、公開中にならず期限切れになる",
				publishedAt: &past,
				expiresAt:   &now,
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
	})
}
