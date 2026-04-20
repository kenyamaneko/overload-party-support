package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
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

// Create は問い合わせを status = new で INSERT する (FEATURE_SPEC §7.1)。
func (r *InquiryRepository) Create(ctx context.Context, title, body, replyEmail string) (*apisupport.Inquiry, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO support.inquiries (title, body, reply_email, status)
		 VALUES ($1, $2, $3, 'new')
		 RETURNING inquiry_id, title, body, reply_email, status, internal_note, created_at, updated_at`,
		title, body, replyEmail,
	)
	return scanInquiry(row)
}

// List は指定 statuses に一致する問い合わせを updated_at DESC 順で返す (FEATURE_SPEC §8.3)。
// 「全件」の表現は呼び出し側が apisupport.Statuses を丸ごと渡して行う (repo は分岐を持たない)。
func (r *InquiryRepository) List(ctx context.Context, statuses []apisupport.Status) ([]apisupport.Inquiry, error) {
	stStr := make([]string, len(statuses))
	for i, s := range statuses {
		stStr[i] = string(s)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT inquiry_id, title, body, reply_email, status, internal_note, created_at, updated_at
		   FROM support.inquiries
		  WHERE status = ANY($1::text[])
		  ORDER BY updated_at DESC, inquiry_id DESC`,
		stStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query inquiries: %w", err)
	}
	defer rows.Close()

	var items []apisupport.Inquiry
	for rows.Next() {
		inq, err := scanInquiry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan inquiry row: %w", err)
		}
		items = append(items, *inq)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inquiries: %w", err)
	}
	return items, nil
}

// Get は ID で単一の問い合わせを返す。非存在なら ErrNotFound。
func (r *InquiryRepository) Get(ctx context.Context, inquiryID int64) (*apisupport.Inquiry, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT inquiry_id, title, body, reply_email, status, internal_note, created_at, updated_at
		   FROM support.inquiries
		  WHERE inquiry_id = $1`,
		inquiryID,
	)
	inq, err := scanInquiry(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("inquiry %d: %w", inquiryID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("query inquiry: %w", err)
	}
	return inq, nil
}

// UpdateStatus は status を更新し、更新後の行を返す。非存在なら ErrNotFound。
func (r *InquiryRepository) UpdateStatus(ctx context.Context, inquiryID int64, newStatus apisupport.Status) (*apisupport.Inquiry, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE support.inquiries
		    SET status = $2
		  WHERE inquiry_id = $1
		  RETURNING inquiry_id, title, body, reply_email, status, internal_note, created_at, updated_at`,
		inquiryID, string(newStatus),
	)
	inq, err := scanInquiry(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("inquiry %d: %w", inquiryID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("update inquiry status: %w", err)
	}
	return inq, nil
}

// UpdateNote は internal_note を更新し、更新後の行を返す。非存在なら ErrNotFound。
func (r *InquiryRepository) UpdateNote(ctx context.Context, inquiryID int64, note *string) (*apisupport.Inquiry, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE support.inquiries
		    SET internal_note = $2
		  WHERE inquiry_id = $1
		  RETURNING inquiry_id, title, body, reply_email, status, internal_note, created_at, updated_at`,
		inquiryID, note,
	)
	inq, err := scanInquiry(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("inquiry %d: %w", inquiryID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("update inquiry note: %w", err)
	}
	return inq, nil
}

// scanInquiry は Inquiry 全カラム分の Scan を共通化する。
func scanInquiry(row pgx.Row) (*apisupport.Inquiry, error) {
	var inq apisupport.Inquiry
	var statusStr string
	if err := row.Scan(
		&inq.InquiryID, &inq.Title, &inq.Body, &inq.ReplyEmail,
		&statusStr, &inq.InternalNote, &inq.CreatedAt, &inq.UpdatedAt,
	); err != nil {
		return nil, err
	}
	inq.Status = apisupport.Status(statusStr)
	return &inq, nil
}
