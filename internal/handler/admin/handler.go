package admin

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	announcementadmin "github.com/kenyamaneko/overload-party-support/internal/usecase/announcement_admin"
)

// datetimeLocalLayout は HTML <input type="datetime-local"> の value 形式 (Go time layout)。
const datetimeLocalLayout = "2006-01-02T15:04"

// adminListItem は一覧画面描画用のビューモデル。
type adminListItem struct {
	Announcement domain.Announcement
	JaTitle      string
	LangsLabel   string // 存在する翻訳言語 (例: "ja, en")
	State        string
}

// Handler は管理 UI の HTTP エンドポイント群を提供する。
type Handler struct {
	uc  *announcementadmin.Usecase
	tpl *templates
	now func() time.Time
}

// NewHandler は Handler を生成する。
func NewHandler(uc *announcementadmin.Usecase, now func() time.Time) (*Handler, error) {
	tpl, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	return &Handler{uc: uc, tpl: tpl, now: now}, nil
}

// List は `GET /admin/announcements` を処理する。
func (h *Handler) List(c *gin.Context) {
	filter := c.Query("status")
	items, err := h.uc.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}

	now := h.now()
	viewItems := make([]adminListItem, 0, len(items))
	for _, it := range items {
		viewItem, err := toAdminListItem(it, now)
		if err != nil {
			respondError(c, err)
			return
		}
		viewItems = append(viewItems, viewItem)
	}

	data := gin.H{
		"Title":        "お知らせ一覧",
		"Reviewer":     Reviewer(c),
		"ActiveStatus": filter,
		"Items":        viewItems,
	}
	renderPage(c, h.tpl.list, data)
}

// ShowNew は `GET /admin/announcements/new` を処理する。
func (h *Handler) ShowNew(c *gin.Context) {
	data := gin.H{
		"Title":    "お知らせ作成",
		"Reviewer": Reviewer(c),
		"Types":    domain.Types,
	}
	renderPage(c, h.tpl.new, data)
}

// Create は `POST /admin/announcements` を処理する。
func (h *Handler) Create(c *gin.Context) {
	publishedAt, err := parseDatetimeLocal(c.PostForm("published_at"))
	if err != nil {
		respondError(c, err)
		return
	}
	expiresAt, err := parseDatetimeLocal(c.PostForm("expires_at"))
	if err != nil {
		respondError(c, err)
		return
	}

	translations, err := collectCreateTranslations(c)
	if err != nil {
		respondError(c, err)
		return
	}

	id, err := h.uc.Create(c.Request.Context(), domain.CreateAnnouncementParams{
		Type:         c.PostForm("type"),
		PublishedAt:  publishedAt,
		ExpiresAt:    expiresAt,
		Translations: translations,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	redirectTo(c, fmt.Sprintf("/admin/announcements/%d", id))
}

// collectCreateTranslations は作成フォームから ja (必須) と en (任意) の翻訳を取り出す。
func collectCreateTranslations(c *gin.Context) ([]domain.TranslationInput, error) {
	translations := []domain.TranslationInput{
		{Lang: domain.LangJa, Title: c.PostForm("ja_title"), Body: c.PostForm("ja_body")},
	}
	enTitle := c.PostForm("en_title")
	enBody := c.PostForm("en_body")
	switch {
	case enTitle == "" && enBody == "":
		return translations, nil
	case enTitle != "" && enBody != "":
		return append(translations, domain.TranslationInput{Lang: domain.LangEn, Title: enTitle, Body: enBody}), nil
	default:
		return nil, fmt.Errorf("en_title and en_body must both be provided: %w", announcementadmin.ErrInvalidField)
	}
}

// ShowEdit は `GET /admin/announcements/:announcementId` を処理する。
func (h *Handler) ShowEdit(c *gin.Context) {
	id, ok := parseAnnouncementID(c)
	if !ok {
		return
	}
	aw, err := h.uc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	title, err := findJaTitle(aw.Translations)
	if err != nil {
		respondError(c, err)
		return
	}
	data := gin.H{
		"Title":        title,
		"Reviewer":     Reviewer(c),
		"Announcement": aw.Announcement,
		"Translations": aw.Translations,
		"Types":        domain.Types,
		"Langs":        domain.SupportedLangs,
	}
	renderPage(c, h.tpl.edit, data)
}

// Update は `POST /admin/announcements/:announcementId` を処理する (本体属性のみ)。
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseAnnouncementID(c)
	if !ok {
		return
	}
	publishedAt, err := parseDatetimeLocal(c.PostForm("published_at"))
	if err != nil {
		respondError(c, err)
		return
	}
	expiresAt, err := parseDatetimeLocal(c.PostForm("expires_at"))
	if err != nil {
		respondError(c, err)
		return
	}

	if err := h.uc.Update(c.Request.Context(), id, domain.UpdateAnnouncementParams{
		Type:        c.PostForm("type"),
		PublishedAt: publishedAt,
		ExpiresAt:   expiresAt,
	}); err != nil {
		respondError(c, err)
		return
	}
	redirectTo(c, "/admin/announcements")
}

// Delete は `POST /admin/announcements/:announcementId/delete` を処理する。
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseAnnouncementID(c)
	if !ok {
		return
	}
	if err := h.uc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	// 一覧行からの HTMX 削除: 行フラグメントを空で返して DOM から除去、
	// それ以外は一覧へリダイレクト。
	if c.GetHeader("HX-Target") == fmt.Sprintf("row-%d", id) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(""))
		return
	}
	redirectTo(c, "/admin/announcements")
}

// UpsertTranslation は `POST /admin/announcements/:announcementId/translations/:lang` を処理する。
func (h *Handler) UpsertTranslation(c *gin.Context) {
	id, ok := parseAnnouncementID(c)
	if !ok {
		return
	}
	lang := c.Param("lang")
	title := c.PostForm("title")
	body := c.PostForm("body")

	if err := h.uc.UpsertTranslation(c.Request.Context(), id, lang, title, body); err != nil {
		respondError(c, err)
		return
	}
	redirectTo(c, fmt.Sprintf("/admin/announcements/%d", id))
}

// parseAnnouncementID はパスから announcement_id を取り出す。不正なら 404 を返して false。
func parseAnnouncementID(c *gin.Context) (int64, bool) {
	idStr := c.Param("announcementId")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.String(http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}

// parseDatetimeLocal は `<input type="datetime-local">` の値 (YYYY-MM-DDTHH:MM) を *time.Time に変換する。
// 空文字は nil (= 下書き / 無期限)。
func parseDatetimeLocal(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation(datetimeLocalLayout, s, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid datetime %q", announcementadmin.ErrInvalidField, s)
	}
	return &t, nil
}

// redirectTo は HTMX リクエストなら HX-Redirect、通常リクエストなら 303 で遷移。
func redirectTo(c *gin.Context, url string) {
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", url)
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, url)
}

// renderPage はレイアウト込みのフルページ HTML を描画する。
func renderPage(c *gin.Context, tpl *template.Template, data any) {
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		respondError(c, fmt.Errorf("render page: %w", err))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// toAdminListItem は AnnouncementWithTranslations から一覧表示用のビューモデルを作る。
func toAdminListItem(aw domain.AnnouncementWithTranslations, now time.Time) (adminListItem, error) {
	jaTitle, err := findJaTitle(aw.Translations)
	if err != nil {
		return adminListItem{}, err
	}
	langs := make([]string, 0, len(aw.Translations))
	for _, t := range aw.Translations {
		langs = append(langs, t.Lang)
	}
	langsLabel := ""
	for i, l := range langs {
		if i > 0 {
			langsLabel += ", "
		}
		langsLabel += l
	}
	return adminListItem{
		Announcement: aw.Announcement,
		JaTitle:      jaTitle,
		LangsLabel:   langsLabel,
		State:        announcementadmin.DeriveState(aw.Announcement, now),
	}, nil
}

// findJaTitle は admin UI の表示用に ja 翻訳のタイトルを返す。
func findJaTitle(translations []domain.Translation) (string, error) {
	for _, t := range translations {
		if t.Lang == domain.LangJa {
			return t.Title, nil
		}
	}
	return "", fmt.Errorf("ja translation missing")
}
