// Package announcementadmin は管理 UI (:9109) のお知らせ CRUD ユースケース層。
// バリデーションと port.AnnouncementAdmin への委譲を担う。
package announcementadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

const (
	titleMaxLen = 200
)

// Usecase は管理 UI のお知らせ CRUD ユースケースを担う。
type Usecase struct {
	admin port.AnnouncementAdmin
	now   func() time.Time
}

// New は Usecase を構築する。
func New(admin port.AnnouncementAdmin, now func() time.Time) *Usecase {
	return &Usecase{admin: admin, now: now}
}

// List は管理 UI の一覧取得 (FEATURE_SPEC §6.1 / §6.3)。
// filter が空文字 / "all" の場合は全件 (state == nil)。
func (u *Usecase) List(ctx context.Context, filter string) ([]domain.AnnouncementWithTranslations, error) {
	state, err := parseStateFilter(filter)
	if err != nil {
		return nil, err
	}
	items, err := u.admin.ListAll(ctx, state, u.now())
	if err != nil {
		return nil, fmt.Errorf("list all: %w", err)
	}
	return items, nil
}

// DeriveState は本体属性から state を導出する (FEATURE_SPEC §6.3)。
// state 境界の SSoT。repo の ListAll が state ごとに用意する WHERE 述語と 1:1 で対応する。
// 判定順序が排他性を担保している (PublishedAt IS NULL を最初に判定するため、
// 例えば PublishedAt=NULL ∧ ExpiresAt<=now の record は Expired ではなく Draft になる)。
func DeriveState(a domain.Announcement, now time.Time) string {
	if a.PublishedAt == nil {
		return domain.StateDraft
	}
	if a.PublishedAt.After(now) {
		return domain.StateScheduled
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return domain.StateExpired
	}
	return domain.StatePublished
}

// Get は ID 指定の編集画面向け取得。
func (u *Usecase) Get(ctx context.Context, announcementID int64) (*domain.AnnouncementWithTranslations, error) {
	got, err := u.admin.GetWithTranslations(ctx, announcementID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get: %w", err)
	}
	return got, nil
}

// Create は新規作成 (FEATURE_SPEC §6.4)。本体 + 翻訳群を同一 tx で INSERT する責務は port 実装側。
// service は入力バリデーション (ja 翻訳必須・各フィールド制約・lang 重複なし) のみ担う。
func (u *Usecase) Create(ctx context.Context, params domain.CreateAnnouncementParams) (int64, error) {
	if !domain.IsSupportedType(params.Type) {
		return 0, ErrInvalidType
	}
	hasJa := false
	seenLangs := make(map[string]struct{}, len(params.Translations))
	for _, t := range params.Translations {
		if err := validateTranslationFields(t.Lang, t.Title, t.Body); err != nil {
			return 0, err
		}
		if _, dup := seenLangs[t.Lang]; dup {
			return 0, ErrInvalidField
		}
		seenLangs[t.Lang] = struct{}{}
		if t.Lang == domain.LangJa {
			hasJa = true
		}
	}
	if !hasJa {
		return 0, ErrInvalidField
	}

	id, err := u.admin.Create(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("create: %w", err)
	}
	return id, nil
}

// Update は本体属性を更新する (FEATURE_SPEC §6.4)。
func (u *Usecase) Update(ctx context.Context, announcementID int64, params domain.UpdateAnnouncementParams) error {
	if !domain.IsSupportedType(params.Type) {
		return ErrInvalidType
	}
	err := u.admin.Update(ctx, announcementID, params)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// Delete はお知らせを削除する。翻訳は FK CASCADE で同時削除。
func (u *Usecase) Delete(ctx context.Context, announcementID int64) error {
	err := u.admin.Delete(ctx, announcementID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// UpsertTranslation は翻訳を INSERT / UPDATE する (FEATURE_SPEC §6.5)。
func (u *Usecase) UpsertTranslation(ctx context.Context, announcementID int64, lang, title, body string) error {
	if err := validateTranslationFields(lang, title, body); err != nil {
		return err
	}
	err := u.admin.UpsertTranslation(ctx, announcementID, lang, title, body)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("upsert translation: %w", err)
	}
	return nil
}

// parseStateFilter は "all" / "draft" / "scheduled" / "published" / "expired" (空は "all") を
// *AnnouncementState に変換する。"" / "all" は nil (= 全件) を返す。
func parseStateFilter(s string) (*string, error) {
	switch s {
	case "", "all":
		return nil, nil
	case "draft":
		st := domain.StateDraft
		return &st, nil
	case "scheduled":
		st := domain.StateScheduled
		return &st, nil
	case "published":
		st := domain.StatePublished
		return &st, nil
	case "expired":
		st := domain.StateExpired
		return &st, nil
	default:
		return nil, ErrInvalidStatusFilter
	}
}

// validateTranslationFields は翻訳 upsert / 新規作成時のフィールド制約 (§6.6)。
func validateTranslationFields(lang, title, body string) error {
	if !domain.IsSupportedLang(lang) {
		return ErrUnsupportedLang
	}
	if title == "" || len([]rune(title)) > titleMaxLen {
		return ErrInvalidField
	}
	if body == "" {
		return ErrInvalidField
	}
	return nil
}
