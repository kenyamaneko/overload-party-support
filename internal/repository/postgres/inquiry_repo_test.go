package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
)

// newInquiryRepo は共有 Postgres を TRUNCATE した上で repo を生成する。
func newInquiryRepo(t *testing.T) *postgres.InquiryRepository {
	t.Helper()
	sharedPG.Truncate(t)
	return postgres.NewInquiryRepository(sharedPG.Pool)
}

func TestInquiryCreate(t *testing.T) {
	t.Run("問い合わせの作成", func(t *testing.T) {
		t.Run("作成すると status=new の行が採番 ID 付きで返り、連続作成で ID が単調増加する", func(t *testing.T) {
			repo := newInquiryRepo(t)
			ctx := context.Background()

			inq1, err := repo.Create(ctx, "T1", "B1", "u1@e.com")
			require.NoError(t, err)
			require.NotNil(t, inq1)
			assert.Equal(t, "new", inq1.Status)
			assert.Equal(t, "T1", inq1.Title)
			assert.Equal(t, "B1", inq1.Body)
			assert.Equal(t, "u1@e.com", inq1.ReplyEmail)
			assert.Nil(t, inq1.InternalNote)

			// 採番は DB の IDENTITY に委譲するため、連続 INSERT で ID は単調増加する。
			inq2, err := repo.Create(ctx, "T2", "B2", "u2@e.com")
			require.NoError(t, err)
			require.NotNil(t, inq2)
			assert.Greater(t, inq2.InquiryID, inq1.InquiryID)
		})
	})
}
