package announcementadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

const (
	titleMaxLen = 200
)

// Usecase は管理 UI のお知らせ CRUD ユースケースを担う。
type Usecase struct {
	repo port.AnnouncementRepository
	now  func() time.Time
}

// New は Usecase を構築する。
func New(repo port.AnnouncementRepository, now func() time.Time) *Usecase {
	return &Usecase{repo: repo, now: now}
}

// Create は新規作成。
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

	id, err := u.repo.Create(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("create: %w", err)
	}
	return id, nil
}

// List は管理 UI の一覧取得。filter が空文字 / "all" の場合は全件。
func (u *Usecase) List(ctx context.Context, filter string) ([]domain.AnnouncementWithTranslations, error) {
	state, err := parseStateFilter(filter)
	if err != nil {
		return nil, err
	}
	items, err := u.repo.List(ctx, state, u.now())
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	return items, nil
}

// Get は ID 指定の編集画面向け取得。
func (u *Usecase) Get(ctx context.Context, announcementID int64) (*domain.AnnouncementWithTranslations, error) {
	got, err := u.repo.GetWithTranslations(ctx, announcementID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get: %w", err)
	}
	return got, nil
}

// Update は本体属性を更新する。
func (u *Usecase) Update(ctx context.Context, announcementID int64, params domain.UpdateAnnouncementParams) error {
	if !domain.IsSupportedType(params.Type) {
		return ErrInvalidType
	}
	err := u.repo.Update(ctx, announcementID, params)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// UpsertTranslation は翻訳を INSERT / UPDATE する。
func (u *Usecase) UpsertTranslation(ctx context.Context, announcementID int64, lang, title, body string) error {
	if err := validateTranslationFields(lang, title, body); err != nil {
		return err
	}
	err := u.repo.UpsertTranslation(ctx, announcementID, lang, title, body)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("upsert translation: %w", err)
	}
	return nil
}

// Delete はお知らせを削除する。翻訳は FK CASCADE で同時削除。
func (u *Usecase) Delete(ctx context.Context, announcementID int64) error {
	err := u.repo.Delete(ctx, announcementID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete: %w", err)
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

// validateTranslationFields は翻訳 upsert / 新規作成時のフィールド制約。
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
