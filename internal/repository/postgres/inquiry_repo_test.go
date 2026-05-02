package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
)

// newInquiryRepo は共有 Postgres を TRUNCATE した上で repo を生成する。
func newInquiryRepo(t *testing.T) *postgres.InquiryRepository {
	t.Helper()
	sharedPG.Truncate(t)
	return postgres.NewInquiryRepository(sharedPG.Pool)
}

// 仕様 (FEATURE_SPEC §7.1): Create は status = new で INSERT し、採番 ID 付きの行を返す。
// 採番は DB の IDENTITY に委譲する (連続 INSERT で単調増加)。
func TestInquiryCreate(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	inq1, err := repo.Create(ctx, "T1", "B1", "u1@e.com")
	require.NoError(t, err)
	require.NotNil(t, inq1)
	assert.Equal(t, domain.StatusNew, inq1.Status)
	assert.Equal(t, "T1", inq1.Title)
	assert.Equal(t, "B1", inq1.Body)
	assert.Equal(t, "u1@e.com", inq1.ReplyEmail)
	assert.Nil(t, inq1.InternalNote)

	inq2, err := repo.Create(ctx, "T2", "B2", "u2@e.com")
	require.NoError(t, err)
	require.NotNil(t, inq2)
	assert.Greater(t, inq2.InquiryID, inq1.InquiryID)
}

// 仕様 (FEATURE_SPEC §8.3): nil ではなく長さ 0 の slice を返す契約 (呼び出し側が range できる)。
func TestInquiryList_Empty(t *testing.T) {
	cases := []struct {
		name     string
		seed     func(t *testing.T, repo *postgres.InquiryRepository, ctx context.Context)
		statuses []string
	}{
		{
			name:     "空 DB に対する全 status 指定",
			seed:     func(t *testing.T, repo *postgres.InquiryRepository, ctx context.Context) {},
			statuses: domain.Statuses,
		},
		{
			name: "DB に status=new はあるが status=in_progress で絞ると一致なし",
			seed: func(t *testing.T, repo *postgres.InquiryRepository, ctx context.Context) {
				_, err := repo.Create(ctx, "only-new", "B", "u@e.com")
				require.NoError(t, err)
			},
			statuses: []string{domain.StatusInProgress},
		},
		{
			name: "statuses=nil は 0 件 (呼び出し側が全件を明示する契約)",
			seed: func(t *testing.T, repo *postgres.InquiryRepository, ctx context.Context) {
				_, err := repo.Create(ctx, "x", "B", "u@e.com")
				require.NoError(t, err)
			},
			statuses: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newInquiryRepo(t)
			ctx := context.Background()
			tc.seed(t, repo, ctx)

			items, err := repo.List(ctx, tc.statuses)
			require.NoError(t, err)
			assert.Empty(t, items)
		})
	}
}

// 仕様 (FEATURE_SPEC §8.3): List は statuses で指定された status の行のみを返す (指定外は混入しない)。
func TestInquiryList_FilterByStatus(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	mustCreate := func(title, body, email string) *domain.Inquiry {
		t.Helper()
		inq, err := repo.Create(ctx, title, body, email)
		require.NoError(t, err)
		return inq
	}
	mustUpdateStatus := func(id int64, status string) {
		t.Helper()
		_, err := repo.UpdateStatus(ctx, id, status)
		require.NoError(t, err)
	}
	mustCreate("n", "B", "n@e.com")
	inqP := mustCreate("p", "B", "p@e.com")
	inqC := mustCreate("c", "B", "c@e.com")
	mustUpdateStatus(inqP.InquiryID, domain.StatusInProgress)
	mustUpdateStatus(inqC.InquiryID, domain.StatusClosed)

	requested := []string{domain.StatusInProgress, domain.StatusClosed}
	items, err := repo.List(ctx, requested)
	require.NoError(t, err)

	for _, it := range items {
		assert.Contains(t, requested, it.Status)
	}
}

// 仕様 (FEATURE_SPEC §8.3): List は updated_at DESC 順で並べる。
func TestInquiryList_Order(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	mustCreate := func(title, body, email string) *domain.Inquiry {
		t.Helper()
		inq, err := repo.Create(ctx, title, body, email)
		require.NoError(t, err)
		return inq
	}
	mustUpdateStatus := func(id int64, status string) {
		t.Helper()
		_, err := repo.UpdateStatus(ctx, id, status)
		require.NoError(t, err)
	}
	inq1 := mustCreate("first", "B", "u1@e.com")
	inq2 := mustCreate("second", "B", "u2@e.com")
	inq3 := mustCreate("third", "B", "u3@e.com")
	mustUpdateStatus(inq2.InquiryID, domain.StatusInProgress)
	mustUpdateStatus(inq3.InquiryID, domain.StatusClosed)

	items, err := repo.List(ctx, domain.Statuses)
	require.NoError(t, err)
	require.Len(t, items, 3)

	gotIDs := make([]int64, len(items))
	for i, it := range items {
		gotIDs[i] = it.InquiryID
	}
	assert.Equal(t, []int64{inq3.InquiryID, inq2.InquiryID, inq1.InquiryID}, gotIDs)
}

// 仕様 (FEATURE_SPEC §8.3): Get は既存 ID に対応する行を返す。
func TestInquiryGet_Existing(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	inq, err := repo.Create(ctx, "T", "B", "u@e.com")
	require.NoError(t, err)

	got, err := repo.Get(ctx, inq.InquiryID)
	require.NoError(t, err)
	assert.Equal(t, inq.InquiryID, got.InquiryID)
	assert.Equal(t, "T", got.Title)
	assert.Equal(t, "B", got.Body)
	assert.Equal(t, "u@e.com", got.ReplyEmail)
	assert.Equal(t, domain.StatusNew, got.Status)
}

// 仕様 (FEATURE_SPEC §8.3): 未存在 ID は port.ErrNotFound。
func TestInquiryGet_NotFound(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, 999999)
	assert.ErrorIs(t, err, port.ErrNotFound)
}

// 仕様 (FEATURE_SPEC §8.1): status CHECK 制約の許容値はすべて DDL レベルで通過する。
// API 側で new への遷移を拒否するかは usecase 層の責務であり、DDL としては許容する。
func TestUpdateStatus_Allowed(t *testing.T) {
	cases := []struct {
		name   string
		status string
	}{
		{
			name:   "new",
			status: domain.StatusNew,
		},
		{
			name:   "in_progress",
			status: domain.StatusInProgress,
		},
		{
			name:   "closed",
			status: domain.StatusClosed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newInquiryRepo(t)
			ctx := context.Background()
			inq, err := repo.Create(ctx, "T", "B", "u@e.com")
			require.NoError(t, err)

			_, err = repo.UpdateStatus(ctx, inq.InquiryID, tc.status)
			assert.NoError(t, err)
		})
	}
}

// 仕様 (FEATURE_SPEC §8.1): 許容値外の status は DDL CHECK で拒否され、DB エラーが返る。
func TestUpdateStatus_Rejected(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()
	inq, err := repo.Create(ctx, "T", "B", "u@e.com")
	require.NoError(t, err)

	_, err = repo.UpdateStatus(ctx, inq.InquiryID, "invalid")
	assert.Error(t, err)
}

// 仕様 (FEATURE_SPEC §8.1): 未存在 ID は port.ErrNotFound。
func TestUpdateStatus_NotFound(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	_, err := repo.UpdateStatus(ctx, 999999, domain.StatusClosed)
	assert.ErrorIs(t, err, port.ErrNotFound)
}

// 仕様 (FEATURE_SPEC §8.2): UpdateNote は渡された note を行に反映する。
func TestUpdateNote_Applies(t *testing.T) {
	noteValue := "調査中"
	prevValue := "prev"

	cases := []struct {
		name  string
		setup func(t *testing.T, repo *postgres.InquiryRepository, ctx context.Context) int64
		note  *string
	}{
		{
			name: "値ありのメモを書き込む (new 作成直後なので prev は nil)",
			setup: func(t *testing.T, repo *postgres.InquiryRepository, ctx context.Context) int64 {
				inq, err := repo.Create(ctx, "T", "B", "u@e.com")
				require.NoError(t, err)
				return inq.InquiryID
			},
			note: &noteValue,
		},
		{
			name: "既存メモを nil で NULL に戻す",
			setup: func(t *testing.T, repo *postgres.InquiryRepository, ctx context.Context) int64 {
				inq, err := repo.Create(ctx, "T", "B", "u@e.com")
				require.NoError(t, err)
				_, err = repo.UpdateNote(ctx, inq.InquiryID, &prevValue)
				require.NoError(t, err)
				return inq.InquiryID
			},
			note: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newInquiryRepo(t)
			ctx := context.Background()
			id := tc.setup(t, repo, ctx)

			got, err := repo.UpdateNote(ctx, id, tc.note)
			require.NoError(t, err)
			assert.Equal(t, tc.note, got.InternalNote)
		})
	}
}

// 仕様 (FEATURE_SPEC §8.2): 未存在 ID は port.ErrNotFound。
func TestUpdateNote_NotFound(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()
	note := "N"

	_, err := repo.UpdateNote(ctx, 999999, &note)
	assert.ErrorIs(t, err, port.ErrNotFound)
}
