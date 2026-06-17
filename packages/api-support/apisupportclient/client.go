// Package apisupportclient は consumer から wire 詳細 (REST path / HTTP status code) を
// 隠蔽するため、生成 client を sentinel error 変換でラップする。
package apisupportclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// Sentinel errors. wire 契約 (OpenAPI spec) で定義された status code の意味を sentinel として export する。
var (
	// ErrNotFound は status 404。
	ErrNotFound = errors.New("apisupportclient: not found")

	// ErrUnauthorized は status 401。
	ErrUnauthorized = errors.New("apisupportclient: unauthorized")

	// ErrForbidden は status 403。
	ErrForbidden = errors.New("apisupportclient: forbidden")

	// ErrBadRequest は status 400。
	ErrBadRequest = errors.New("apisupportclient: bad request")

	// ErrInternalServer は status 5xx。
	ErrInternalServer = errors.New("apisupportclient: internal server error")
)

// Client は overload-party-support REST API の sentinel-error converted client。
type Client struct {
	api *apisupport.ClientWithResponses
}

// Option は Client 構築時の設定。
type Option func(*config)

type config struct {
	httpClient apisupport.HttpRequestDoer
	editors    []apisupport.RequestEditorFn
}

// WithHTTPClient は基底 HTTP doer を差し替える。デフォルトは http.DefaultClient。
func WithHTTPClient(doer apisupport.HttpRequestDoer) Option {
	return func(c *config) { c.httpClient = doer }
}

// WithRequestEditorFn は全リクエストに適用する RequestEditor を追加する。
func WithRequestEditorFn(fn apisupport.RequestEditorFn) Option {
	return func(c *config) { c.editors = append(c.editors, fn) }
}

// New は baseURL に接続する Client を生成する。
func New(baseURL string, opts ...Option) (*Client, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	apiOpts := make([]apisupport.ClientOption, 0, 1+len(cfg.editors))
	if cfg.httpClient != nil {
		apiOpts = append(apiOpts, apisupport.WithHTTPClient(cfg.httpClient))
	}
	for _, ed := range cfg.editors {
		apiOpts = append(apiOpts, apisupport.WithRequestEditorFn(ed))
	}

	api, err := apisupport.NewClientWithResponses(baseURL, apiOpts...)
	if err != nil {
		return nil, fmt.Errorf("apisupportclient: new: %w", err)
	}
	return &Client{api: api}, nil
}

// GetHealth はサービス死活監視用エンドポイントを叩く。
func (c *Client) GetHealth(ctx context.Context) (*apisupport.HealthResponse, error) {
	resp, err := c.api.GetHealthWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("apisupportclient: GetHealth: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("GetHealth", resp.StatusCode())
}

// ListAnnouncements は公開中のお知らせ一覧を返す。lang は表示言語コード。
func (c *Client) ListAnnouncements(ctx context.Context, lang string) (*apisupport.AnnouncementListResponse, error) {
	resp, err := c.api.ListAnnouncementsWithResponse(ctx, &apisupport.ListAnnouncementsParams{Lang: lang})
	if err != nil {
		return nil, fmt.Errorf("apisupportclient: ListAnnouncements: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("ListAnnouncements", resp.StatusCode())
}

// GetAnnouncement は ID 指定でお知らせ詳細を返す。
func (c *Client) GetAnnouncement(ctx context.Context, announcementID int64, lang string) (*apisupport.AnnouncementDetail, error) {
	resp, err := c.api.GetAnnouncementWithResponse(ctx, announcementID, &apisupport.GetAnnouncementParams{Lang: lang})
	if err != nil {
		return nil, fmt.Errorf("apisupportclient: GetAnnouncement: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("GetAnnouncement", resp.StatusCode())
}

// SubmitInquiry は問い合わせフォームから受信した内容を送信する。
func (c *Client) SubmitInquiry(ctx context.Context, req apisupport.SubmitInquiryRequest) (*apisupport.SubmitInquiryResult, error) {
	resp, err := c.api.SubmitInquiryWithResponse(ctx, apisupport.SubmitInquiryJSONRequestBody(req))
	if err != nil {
		return nil, fmt.Errorf("apisupportclient: SubmitInquiry: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("SubmitInquiry", resp.StatusCode())
}

// newStatusError は HTTP status code から sentinel error (errors.Is 分岐可能) を生成する。
func newStatusError(op string, code int) error {
	var sentinel error
	switch {
	case code == http.StatusUnauthorized:
		sentinel = ErrUnauthorized
	case code == http.StatusForbidden:
		sentinel = ErrForbidden
	case code == http.StatusNotFound:
		sentinel = ErrNotFound
	case code == http.StatusBadRequest:
		sentinel = ErrBadRequest
	case code >= http.StatusInternalServerError:
		sentinel = ErrInternalServer
	default:
		return fmt.Errorf("apisupportclient: %s: unexpected status %d", op, code)
	}
	return fmt.Errorf("apisupportclient: %s: %w", op, sentinel)
}
