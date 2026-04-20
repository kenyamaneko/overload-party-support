// Package postgres は port で定義されたリポジトリインタフェースの PostgreSQL 実装を提供する。
// pgxpool を共有し、pure data access 層として振る舞う (ビジネスロジックを持たない)。
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// compile-time assertion: AnnouncementRepository が 2 つの port を満たす。
var (
	_ port.AnnouncementQuerier = (*AnnouncementRepository)(nil)
	_ port.AnnouncementAdmin   = (*AnnouncementRepository)(nil)
)

// AnnouncementRepository は support.announcements + announcement_translations への CRUD を提供する。
type AnnouncementRepository struct {
	pool *pgxpool.Pool
}

// NewAnnouncementRepository は pgxpool を受け取り AnnouncementRepository を生成する。
func NewAnnouncementRepository(pool *pgxpool.Pool) *AnnouncementRepository {
	return &AnnouncementRepository{pool: pool}
}

// ListPublished は公開期間内かつ指定 lang の翻訳が存在するお知らせを published_at DESC 順で返す。
// published_at IS NULL (下書き) / published_at > now (予約公開) / expires_at <= now (期限切れ) は除外する。
func (r *AnnouncementRepository) ListPublished(ctx context.Context, lang string, now time.Time) ([]apisupport.AnnouncementSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.announcement_id, a.type, t.title, a.published_at
		   FROM support.announcements a
		   JOIN support.announcement_translations t
		     ON t.announcement_id = a.announcement_id AND t.lang = $1
		  WHERE a.published_at IS NOT NULL
		    AND a.published_at <= $2
		    AND (a.expires_at IS NULL OR a.expires_at > $2)
		  ORDER BY a.published_at DESC, a.announcement_id DESC`,
		lang, now,
	)
	if err != nil {
		return nil, fmt.Errorf("query published announcements: %w", err)
	}
	defer rows.Close()

	var items []apisupport.AnnouncementSummary
	for rows.Next() {
		var item apisupport.AnnouncementSummary
		var typeStr string
		if err := rows.Scan(&item.AnnouncementID, &typeStr, &item.Title, &item.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan published announcement row: %w", err)
		}
		item.Type = apisupport.Type(typeStr)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published announcements: %w", err)
	}
	return items, nil
}

// GetPublishedDetail は ID + lang で単一のお知らせ詳細を返す。
// 公開期間外・下書きでも返す (FEATURE_SPEC §5.2)。翻訳が存在しなければ ErrNotFound。
func (r *AnnouncementRepository) GetPublishedDetail(ctx context.Context, announcementID int64, lang string) (*apisupport.AnnouncementDetail, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT a.announcement_id, a.type, t.title, t.body, a.published_at
		   FROM support.announcements a
		   JOIN support.announcement_translations t
		     ON t.announcement_id = a.announcement_id AND t.lang = $2
		  WHERE a.announcement_id = $1`,
		announcementID, lang,
	)
	var d apisupport.AnnouncementDetail
	var typeStr string
	if err := row.Scan(&d.AnnouncementID, &typeStr, &d.Title, &d.Body, &d.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("announcement %d (lang=%s): %w", announcementID, lang, port.ErrNotFound)
		}
		return nil, fmt.Errorf("query announcement detail: %w", err)
	}
	d.Type = apisupport.Type(typeStr)
	return &d, nil
}

// ListAll は管理 UI 向けに status フィルタ (draft/scheduled/published/expired/all) に応じた一覧 + 全翻訳を返す。
func (r *AnnouncementRepository) ListAll(ctx context.Context, filter port.StatusFilter, now time.Time) ([]apisupport.AnnouncementWithTranslations, error) {
	where, args := statusFilterWhere(filter, now)

	query := `SELECT announcement_id, type, published_at, expires_at, created_at, updated_at
	            FROM support.announcements`
	if where != "" {
		query += " WHERE " + where
	}
	query += ` ORDER BY COALESCE(published_at, created_at) DESC, announcement_id DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query announcements by filter: %w", err)
	}
	defer rows.Close()

	var announcements []apisupport.Announcement
	announcementIDs := make([]int64, 0)
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, fmt.Errorf("scan announcement row: %w", err)
		}
		announcements = append(announcements, *a)
		announcementIDs = append(announcementIDs, a.AnnouncementID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate announcements: %w", err)
	}

	translationsByID, err := r.fetchTranslationsForIDs(ctx, announcementIDs)
	if err != nil {
		return nil, err
	}

	results := make([]apisupport.AnnouncementWithTranslations, 0, len(announcements))
	for _, a := range announcements {
		results = append(results, apisupport.AnnouncementWithTranslations{
			Announcement: a,
			Translations: translationsByID[a.AnnouncementID],
		})
	}
	return results, nil
}

// Get は単一のお知らせ + 全翻訳を返す。非存在なら ErrNotFound。
func (r *AnnouncementRepository) Get(ctx context.Context, announcementID int64) (*apisupport.AnnouncementWithTranslations, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT announcement_id, type, published_at, expires_at, created_at, updated_at
		   FROM support.announcements
		  WHERE announcement_id = $1`,
		announcementID,
	)
	a, err := scanAnnouncement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("announcement %d: %w", announcementID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("query announcement: %w", err)
	}

	translationsByID, err := r.fetchTranslationsForIDs(ctx, []int64{announcementID})
	if err != nil {
		return nil, err
	}
	return &apisupport.AnnouncementWithTranslations{
		Announcement: *a,
		Translations: translationsByID[announcementID],
	}, nil
}

// Create は本体 + 翻訳群を同一 tx で INSERT し、採番された announcement_id を返す (FEATURE_SPEC §6.4)。
// どの言語を含めるかは service が決定し、repo は受け取った翻訳をそのまま INSERT する。
func (r *AnnouncementRepository) Create(ctx context.Context, params port.CreateAnnouncementParams) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO support.announcements (type, published_at, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING announcement_id`,
		string(params.Type), params.PublishedAt, params.ExpiresAt,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert announcement: %w", err)
	}

	for _, t := range params.Translations {
		if _, err := tx.Exec(ctx,
			`INSERT INTO support.announcement_translations (announcement_id, lang, title, body)
			 VALUES ($1, $2, $3, $4)`,
			id, t.Lang, t.Title, t.Body,
		); err != nil {
			return 0, fmt.Errorf("insert %s translation: %w", t.Lang, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return id, nil
}

// Update は本体属性 (type / published_at / expires_at) を更新する。翻訳は触らない。
func (r *AnnouncementRepository) Update(ctx context.Context, announcementID int64, params port.UpdateAnnouncementParams) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE support.announcements
		    SET type = $2,
		        published_at = $3,
		        expires_at = $4
		  WHERE announcement_id = $1`,
		announcementID, string(params.Type), params.PublishedAt, params.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("update announcement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("announcement %d: %w", announcementID, port.ErrNotFound)
	}
	return nil
}

// Delete はお知らせを削除する。翻訳は FK CASCADE で同時削除される。
func (r *AnnouncementRepository) Delete(ctx context.Context, announcementID int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM support.announcements WHERE announcement_id = $1`,
		announcementID,
	)
	if err != nil {
		return fmt.Errorf("delete announcement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("announcement %d: %w", announcementID, port.ErrNotFound)
	}
	return nil
}

// UpsertTranslation は翻訳を INSERT / UPDATE する (FEATURE_SPEC §6.5)。
// 親記事が存在しない場合は FK 違反が先に出るため、事前に存在確認してから実行する。
func (r *AnnouncementRepository) UpsertTranslation(ctx context.Context, announcementID int64, lang, title, body string) error {
	if err := r.announcementExists(ctx, announcementID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO support.announcement_translations (announcement_id, lang, title, body)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (announcement_id, lang) DO UPDATE
		    SET title = EXCLUDED.title,
		        body  = EXCLUDED.body`,
		announcementID, lang, title, body,
	)
	if err != nil {
		return fmt.Errorf("upsert translation: %w", err)
	}
	return nil
}

// announcementExists は親記事の存在を確認する (FK 違反を ErrNotFound に変換)。
func (r *AnnouncementRepository) announcementExists(ctx context.Context, announcementID int64) error {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM support.announcements WHERE announcement_id = $1)`,
		announcementID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check announcement exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("announcement %d: %w", announcementID, port.ErrNotFound)
	}
	return nil
}

// fetchTranslationsForIDs は指定 announcement_id 集合の翻訳を 1 クエリで取得し、
// announcement_id キーでグループ化して返す (N+1 回避)。
func (r *AnnouncementRepository) fetchTranslationsForIDs(ctx context.Context, ids []int64) (map[int64][]apisupport.Translation, error) {
	if len(ids) == 0 {
		return map[int64][]apisupport.Translation{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT announcement_id, lang, title, body, updated_at
		   FROM support.announcement_translations
		  WHERE announcement_id = ANY($1)
		  ORDER BY announcement_id, lang`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("query translations: %w", err)
	}
	defer rows.Close()

	grouped := make(map[int64][]apisupport.Translation)
	for rows.Next() {
		var t apisupport.Translation
		if err := rows.Scan(&t.AnnouncementID, &t.Lang, &t.Title, &t.Body, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan translation: %w", err)
		}
		grouped[t.AnnouncementID] = append(grouped[t.AnnouncementID], t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate translations: %w", err)
	}
	return grouped, nil
}

// scanAnnouncement は Announcement 全カラム分の Scan を共通化する。
func scanAnnouncement(row pgx.Row) (*apisupport.Announcement, error) {
	var a apisupport.Announcement
	var typeStr string
	if err := row.Scan(&a.AnnouncementID, &typeStr, &a.PublishedAt, &a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	a.Type = apisupport.Type(typeStr)
	return &a, nil
}

// statusFilterWhere は管理 UI の status フィルタを WHERE 句に変換する。
// 返す $1 / $2 placeholder の args 配列と対になる。
func statusFilterWhere(filter port.StatusFilter, now time.Time) (string, []any) {
	switch filter {
	case port.StatusFilterDraft:
		return "published_at IS NULL", nil
	case port.StatusFilterScheduled:
		return "published_at IS NOT NULL AND published_at > $1", []any{now}
	case port.StatusFilterPublished:
		return "published_at IS NOT NULL AND published_at <= $1 AND (expires_at IS NULL OR expires_at > $1)", []any{now}
	case port.StatusFilterExpired:
		return "expires_at IS NOT NULL AND expires_at <= $1", []any{now}
	default:
		// StatusFilterAll or unknown → no filter
		return "", nil
	}
}
