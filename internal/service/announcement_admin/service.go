// Package announcementadmin は管理 UI (:9109) のお知らせ CRUD ユースケース層。
// バリデーションと port.AnnouncementAdmin への委譲を担う。
package announcementadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

const (
	titleMaxLen = 200
)

// Service は管理 UI のお知らせ CRUD ユースケースを担う。
type Service struct {
	admin port.AnnouncementAdmin
	now   func() time.Time
}

// New は Service を構築する。
func New(admin port.AnnouncementAdmin, now func() time.Time) *Service {
	return &Service{admin: admin, now: now}
}

// List は管理 UI の一覧取得 (FEATURE_SPEC §6.1 / §6.3)。
// filter が空文字の場合は "all" として扱う。
func (s *Service) List(ctx context.Context, filter string) ([]apisupport.AnnouncementWithTranslations, error) {
	f, err := parseStatusFilter(filter)
	if err != nil {
		return nil, err
	}
	items, err := s.admin.ListAll(ctx, f, s.now())
	if err != nil {
		return nil, fmt.Errorf("list all: %w", err)
	}
	return items, nil
}

// Get は ID 指定の編集画面向け取得。
func (s *Service) Get(ctx context.Context, announcementID int64) (*apisupport.AnnouncementWithTranslations, error) {
	got, err := s.admin.Get(ctx, announcementID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get: %w", err)
	}
	return got, nil
}

// CreateParams は Create の入力。
type CreateParams struct {
	Type         string
	PublishedAt  *time.Time
	ExpiresAt    *time.Time
	Translations []TranslationInput
}

// TranslationInput は Create 時に同一 tx で INSERT する翻訳 1 件分。
type TranslationInput struct {
	Lang  string
	Title string
	Body  string
}

// Create は新規作成 (FEATURE_SPEC §6.4)。本体 + 翻訳群を同一 tx で INSERT する責務は port 実装側。
// service は入力バリデーション (ja 翻訳必須・各フィールド制約・lang 重複なし) のみ担う。
func (s *Service) Create(ctx context.Context, params CreateParams) (int64, error) {
	if !apisupport.IsSupportedType(params.Type) {
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
		if t.Lang == apisupport.LangJa {
			hasJa = true
		}
	}
	if !hasJa {
		return 0, ErrInvalidField
	}

	portTranslations := make([]port.TranslationInput, 0, len(params.Translations))
	for _, t := range params.Translations {
		portTranslations = append(portTranslations, port.TranslationInput{Lang: t.Lang, Title: t.Title, Body: t.Body})
	}
	id, err := s.admin.Create(ctx, port.CreateAnnouncementParams{
		Type:         apisupport.Type(params.Type),
		PublishedAt:  params.PublishedAt,
		ExpiresAt:    params.ExpiresAt,
		Translations: portTranslations,
	})
	if err != nil {
		return 0, fmt.Errorf("create: %w", err)
	}
	return id, nil
}

// UpdateParams は Update の入力 (本体属性のみ、翻訳は UpsertTranslation 経由)。
type UpdateParams struct {
	Type        string
	PublishedAt *time.Time
	ExpiresAt   *time.Time
}

// Update は本体属性を更新する (FEATURE_SPEC §6.4)。
func (s *Service) Update(ctx context.Context, announcementID int64, params UpdateParams) error {
	if !apisupport.IsSupportedType(params.Type) {
		return ErrInvalidType
	}
	err := s.admin.Update(ctx, announcementID, port.UpdateAnnouncementParams{
		Type:        apisupport.Type(params.Type),
		PublishedAt: params.PublishedAt,
		ExpiresAt:   params.ExpiresAt,
	})
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// Delete はお知らせを削除する。翻訳は FK CASCADE で同時削除。
func (s *Service) Delete(ctx context.Context, announcementID int64) error {
	err := s.admin.Delete(ctx, announcementID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// UpsertTranslation は翻訳を INSERT / UPDATE する (FEATURE_SPEC §6.5)。
func (s *Service) UpsertTranslation(ctx context.Context, announcementID int64, lang, title, body string) error {
	if err := validateTranslationFields(lang, title, body); err != nil {
		return err
	}
	err := s.admin.UpsertTranslation(ctx, announcementID, lang, title, body)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("upsert translation: %w", err)
	}
	return nil
}

// parseStatusFilter は "all" / "draft" / "scheduled" / "published" / "expired" (空は "all") を port の StatusFilter に変換する。
func parseStatusFilter(s string) (port.StatusFilter, error) {
	switch s {
	case "", "all":
		return port.StatusFilterAll, nil
	case "draft":
		return port.StatusFilterDraft, nil
	case "scheduled":
		return port.StatusFilterScheduled, nil
	case "published":
		return port.StatusFilterPublished, nil
	case "expired":
		return port.StatusFilterExpired, nil
	default:
		return "", ErrInvalidStatusFilter
	}
}

// validateTranslationFields は翻訳 upsert / 新規作成時のフィールド制約 (§6.6)。
func validateTranslationFields(lang, title, body string) error {
	if !apisupport.IsSupportedLang(lang) {
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
