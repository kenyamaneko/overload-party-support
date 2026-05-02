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

// 仕様 (FEATURE_SPEC §7.1): Create は status = new で INSERT し、採番 ID を含む行を返す。
// 採番は DB の IDENTITY に委譲しており、連続 INSERT で後続の方が大きい ID を得ることで単調増加を確認する。
func TestInquiryCreate_仕様(t *testing.T) {
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
	assert.Greater(t, inq2.InquiryID, inq1.InquiryID, "後続 INSERT は先行よりも大きい ID を得る (単調増加)")
}

// 仕様 (FEATURE_SPEC §8.3): List は空集合を返す契約 (nil ではなく長さ 0 の slice)。
// 空 DB / 指定 status に一致する行が無い / statuses=nil (呼び出し側が全件展開する契約) のいずれでも同じ。
func TestInquiryList_仕様_空集合(t *testing.T) {
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
// 件数の一致ではなく「返却された各行の status が指定集合に含まれる」観点で確認する。
func TestInquiryList_仕様_指定外statusは混入しない(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	// seed: 3 status をそれぞれ 1 件 (status=new / in_progress / closed)
	_, err := repo.Create(ctx, "n", "B", "n@e.com")
	require.NoError(t, err)
	inqP, err := repo.Create(ctx, "p", "B", "p@e.com")
	require.NoError(t, err)
	inqC, err := repo.Create(ctx, "c", "B", "c@e.com")
	require.NoError(t, err)
	_, err = repo.UpdateStatus(ctx, inqP.InquiryID, domain.StatusInProgress)
	require.NoError(t, err)
	_, err = repo.UpdateStatus(ctx, inqC.InquiryID, domain.StatusClosed)
	require.NoError(t, err)

	requested := []string{domain.StatusInProgress, domain.StatusClosed}
	items, err := repo.List(ctx, requested)
	require.NoError(t, err)

	for _, it := range items {
		assert.Contains(t, requested, it.Status)
	}
}

// 仕様 (FEATURE_SPEC §8.3): List は updated_at DESC 順で並べる。
// 直近 UPDATE された順 (inq3 → inq2 → inq1) を slice 比較 1 発で検証する。
func TestInquiryList_仕様_並び順(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	// seed: 受付順 inq1 → inq2 → inq3。その後 inq2 を in_progress、inq3 を closed に更新。
	inq1, err := repo.Create(ctx, "first", "B", "u1@e.com")
	require.NoError(t, err)
	inq2, err := repo.Create(ctx, "second", "B", "u2@e.com")
	require.NoError(t, err)
	inq3, err := repo.Create(ctx, "third", "B", "u3@e.com")
	require.NoError(t, err)
	_, err = repo.UpdateStatus(ctx, inq2.InquiryID, domain.StatusInProgress)
	require.NoError(t, err)
	_, err = repo.UpdateStatus(ctx, inq3.InquiryID, domain.StatusClosed)
	require.NoError(t, err)

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
func TestInquiryGet_仕様_既存ID(t *testing.T) {
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

// 仕様 (FEATURE_SPEC §8.3): Get は未存在で port.ErrNotFound。
func TestInquiryGet_仕様_未存在(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, 999999)
	assert.ErrorIs(t, err, port.ErrNotFound)
}

// 仕様 (FEATURE_SPEC §8.1): status CHECK 制約の許容値 (new / in_progress / closed) はすべて DDL レベルで通過する。
// API 側で new への遷移を拒否するかは service 層の責務であり、DDL としては許容する。
func TestUpdateStatus_仕様_許容値はCHECKを通過(t *testing.T) {
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
func TestUpdateStatus_仕様_許容値外はDB拒否(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()
	inq, err := repo.Create(ctx, "T", "B", "u@e.com")
	require.NoError(t, err)

	_, err = repo.UpdateStatus(ctx, inq.InquiryID, "invalid")
	assert.Error(t, err)
}

// 仕様 (FEATURE_SPEC §8.1): 未存在 ID への UpdateStatus は port.ErrNotFound を返す。
func TestUpdateStatus_仕様_未存在IDはErrNotFound(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	_, err := repo.UpdateStatus(ctx, 999999, domain.StatusClosed)
	assert.ErrorIs(t, err, port.ErrNotFound)
}

// 仕様 (FEATURE_SPEC §8.2): UpdateNote は渡された note を行に反映する。値あり / nil (NULL に戻す) のどちらも対応。
func TestUpdateNote_仕様_noteを反映(t *testing.T) {
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

// 仕様 (FEATURE_SPEC §8.2): 未存在 ID への UpdateNote は port.ErrNotFound を返す。
func TestUpdateNote_仕様_未存在IDはErrNotFound(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()
	note := "N"

	_, err := repo.UpdateNote(ctx, 999999, &note)
	assert.ErrorIs(t, err, port.ErrNotFound)
}
