package postgres

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

var _ port.InquiryStore = (*InquiryRepository)(nil)

// InquiryRepository は support.inquiries への CRUD を提供する。
type InquiryRepository struct {
	pool *pgxpool.Pool
}

// NewInquiryRepository は pgxpool を受け取り InquiryRepository を生成する。
func NewInquiryRepository(pool *pgxpool.Pool) *InquiryRepository {
	return &InquiryRepository{pool: pool}
}

// Create は問い合わせを status = new で INSERT する。
func (r *InquiryRepository) Create(ctx context.Context, title, body, replyEmail string) (*domain.Inquiry, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO support.inquiries (title, body, reply_email, status)
		 VALUES ($1, $2, $3, 'new')
		 RETURNING inquiry_id, title, body, reply_email, status, internal_note, created_at, updated_at`,
		title, body, replyEmail,
	)
	return scanInquiry(row)
}

// scanInquiry は Inquiry 全カラム分の Scan を共通化する。
func scanInquiry(row pgx.Row) (*domain.Inquiry, error) {
	var inquiry domain.Inquiry
	var statusStr string
	if err := row.Scan(
		&inquiry.InquiryID, &inquiry.Title, &inquiry.Body, &inquiry.ReplyEmail,
		&statusStr, &inquiry.InternalNote, &inquiry.CreatedAt, &inquiry.UpdatedAt,
	); err != nil {
		return nil, err
	}
	inquiry.Status = statusStr
	return &inquiry, nil
}
