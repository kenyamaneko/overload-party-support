// Package apisupportserverfake は support の HTTP 契約を実装する httptest.Server ラッパー。
// 各 endpoint は Fn field (func callback) で応答を制御し、Fn=nil なら既定値を返す。
package apisupportserverfake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// Server は support HTTP 契約を実装する httptest.Server wrapper。
type Server struct {
	mu  sync.Mutex
	srv *httptest.Server

	// ListAnnouncementsFn は GET /internal/v1/announcements?lang=<code> の応答を決定する (nil は 200 + 空配列)。
	ListAnnouncementsFn func(lang string) (int, any)

	// GetAnnouncementFn は GET /internal/v1/announcements/{announcementID}?lang=<code> の応答を決定する (nil は 404)。
	GetAnnouncementFn func(announcementID int64, lang string) (int, any)

	// SubmitInquiryFn は POST /api/v1/inquiries の応答を決定する (nil は 200 + 空 Response)。
	SubmitInquiryFn func(req apisupport.SubmitInquiryRequest) (int, any)
}

// NewServer は起動済み Server を返す。テスト終了時に Close() すること。
func NewServer() *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/v1/announcements", s.handleListAnnouncements)
	mux.HandleFunc("GET /internal/v1/announcements/{announcementID}", s.handleGetAnnouncement)
	mux.HandleFunc("POST /api/v1/inquiries", s.handleSubmitInquiry)
	s.srv = httptest.NewServer(mux)
	return s
}

// URL は httptest.Server のベース URL を返す。
func (s *Server) URL() string { return s.srv.URL }

// Close は内部 httptest.Server を閉じる。
func (s *Server) Close() { s.srv.Close() }

func (s *Server) handleListAnnouncements(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.ListAnnouncementsFn
	s.mu.Unlock()

	lang := r.URL.Query().Get("lang")
	if fn == nil {
		writeJSON(w, http.StatusOK, apisupport.AnnouncementListResponse{
			Announcements: []apisupport.AnnouncementSummary{},
		})
		return
	}
	status, body := fn(lang)
	writeJSON(w, status, body)
}

func (s *Server) handleGetAnnouncement(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.GetAnnouncementFn
	s.mu.Unlock()

	announcementID, err := strconv.ParseInt(r.PathValue("announcementID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "announcement not found"})
		return
	}
	lang := r.URL.Query().Get("lang")
	if fn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "announcement not found"})
		return
	}
	status, body := fn(announcementID, lang)
	writeJSON(w, status, body)
}

func (s *Server) handleSubmitInquiry(w http.ResponseWriter, r *http.Request) {
	var req apisupport.SubmitInquiryRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	s.mu.Lock()
	fn := s.SubmitInquiryFn
	s.mu.Unlock()

	if fn == nil {
		writeJSON(w, http.StatusOK, apisupport.SubmitInquiryResponse{})
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

// writeJSON は status と body を書く (body=nil なら status のみ)。
func writeJSON(w http.ResponseWriter, status int, body any) {
	if body == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
