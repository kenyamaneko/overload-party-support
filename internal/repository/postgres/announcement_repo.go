package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

// compile-time assertion: AnnouncementRepository が port を満たす。
var _ port.AnnouncementQuerier = (*AnnouncementRepository)(nil)

// AnnouncementRepository は support.announcements + announcement_translations への参照を提供する。
type AnnouncementRepository struct {
	pool *pgxpool.Pool
}

// NewAnnouncementRepository は pgxpool を受け取り AnnouncementRepository を生成する。
func NewAnnouncementRepository(pool *pgxpool.Pool) *AnnouncementRepository {
	return &AnnouncementRepository{pool: pool}
}

// ListPublished は公開期間内かつ指定 lang の翻訳が存在するお知らせを published_at DESC 順で返す。
func (r *AnnouncementRepository) ListPublished(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.announcement_id, a.type, t.title, a.published_at
		   FROM support.announcements a
		   INNER JOIN support.announcement_translations t
		     ON t.announcement_id = a.announcement_id AND t.lang = $2
		  WHERE a.published_at IS NOT NULL
		    AND a.published_at <= $1
		    AND (a.expires_at IS NULL OR a.expires_at > $1)
		  ORDER BY a.published_at DESC, a.announcement_id DESC`,
		now, lang,
	)
	if err != nil {
		return nil, fmt.Errorf("query published announcements: %w", err)
	}
	defer rows.Close()

	var items []domain.AnnouncementSummary
	for rows.Next() {
		var item domain.AnnouncementSummary
		if err := rows.Scan(&item.AnnouncementID, &item.Type, &item.Title, &item.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan published announcement row: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published announcements: %w", err)
	}
	return items, nil
}

// GetPublishedDetail は ID + lang で単一のお知らせ詳細を返す (公開期間外・下書きでも返す)。
// 翻訳が存在しなければ ErrNotFound。
func (r *AnnouncementRepository) GetPublishedDetail(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT a.announcement_id, a.type, t.title, t.body, a.published_at
		   FROM support.announcements a
		   INNER JOIN support.announcement_translations t
		     ON t.announcement_id = a.announcement_id AND t.lang = $2
		  WHERE a.announcement_id = $1`,
		announcementID, lang,
	)
	var d domain.AnnouncementDetail
	if err := row.Scan(&d.AnnouncementID, &d.Type, &d.Title, &d.Body, &d.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("announcement %d (lang=%s): %w", announcementID, lang, port.ErrNotFound)
		}
		return nil, fmt.Errorf("query announcement detail: %w", err)
	}
	return &d, nil
}
