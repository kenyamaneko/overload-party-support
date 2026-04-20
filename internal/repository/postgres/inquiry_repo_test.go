package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
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
	assert.Equal(t, apisupport.StatusNew, inq1.Status)
	assert.Equal(t, "T1", inq1.Title)
	assert.Equal(t, "B1", inq1.Body)
	assert.Equal(t, "u1@e.com", inq1.ReplyEmail)
	assert.Nil(t, inq1.InternalNote)

	inq2, err := repo.Create(ctx, "T2", "B2", "u2@e.com")
	require.NoError(t, err)
	require.NotNil(t, inq2)
	assert.Greater(t, inq2.InquiryID, inq1.InquiryID, "後続 INSERT は先行よりも大きい ID を得る (単調増加)")
}

// 仕様 (FEATURE_SPEC §8.3): 空 DB に対する List はエラーなく空 slice を返す。
// nil ではなく長さ 0 の slice を返すこと (呼び出し側が range できる契約)。
func TestInquiryList_仕様_空DB(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	items, err := repo.List(ctx, apisupport.Statuses)
	require.NoError(t, err)
	assert.Empty(t, items)
}

// 仕様 (FEATURE_SPEC §8.3): List は status フィルタを反映した件数で返す。
func TestInquiryList_仕様_フィルタ(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	// seed: 3 件受付 → 2 件目を in_progress、3 件目を closed に遷移 (status 別に 1 件ずつ)
	_, err := repo.Create(ctx, "first", "B", "u1@e.com")
	require.NoError(t, err)
	inq2, err := repo.Create(ctx, "second", "B", "u2@e.com")
	require.NoError(t, err)
	inq3, err := repo.Create(ctx, "third", "B", "u3@e.com")
	require.NoError(t, err)

	_, err = repo.UpdateStatus(ctx, inq2.InquiryID, apisupport.StatusInProgress)
	require.NoError(t, err)
	_, err = repo.UpdateStatus(ctx, inq3.InquiryID, apisupport.StatusClosed)
	require.NoError(t, err)

	cases := []struct {
		name      string
		statuses  []apisupport.Status
		wantCount int
	}{
		{
			name:      "3 種すべて指定 = 全件",
			statuses:  apisupport.Statuses,
			wantCount: 3,
		},
		{
			name:      "new + in_progress",
			statuses:  []apisupport.Status{apisupport.StatusNew, apisupport.StatusInProgress},
			wantCount: 2,
		},
		{
			name:      "closed のみ",
			statuses:  []apisupport.Status{apisupport.StatusClosed},
			wantCount: 1,
		},
		{
			name:      "空 slice は 0 件 (呼び出し側が全件を明示する契約)",
			statuses:  nil,
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := repo.List(ctx, tc.statuses)
			require.NoError(t, err)
			assert.Len(t, items, tc.wantCount)
		})
	}
}

// 仕様 (FEATURE_SPEC §8.3): 他 status のレコードは存在するが、絞り込んだ status に一致する行が無い場合は空 slice を返す。
// seed は status=new を 1 件のみ残し、一致しない status で絞ると 0 件になることを確認する。
func TestInquiryList_仕様_一致件数0(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, "only-new", "B", "u@e.com")
	require.NoError(t, err)

	cases := []struct {
		name      string
		statuses  []apisupport.Status
		wantCount int
	}{
		{
			name:      "status=new で絞ると 1 件 (sanity)",
			statuses:  []apisupport.Status{apisupport.StatusNew},
			wantCount: 1,
		},
		{
			name:      "status=in_progress で絞ると 0 件 (DB に該当行が無い)",
			statuses:  []apisupport.Status{apisupport.StatusInProgress},
			wantCount: 0,
		},
		{
			name:      "status=closed で絞ると 0 件 (DB に該当行が無い)",
			statuses:  []apisupport.Status{apisupport.StatusClosed},
			wantCount: 0,
		},
		{
			name:      "status=in_progress + closed でも 0 件",
			statuses:  []apisupport.Status{apisupport.StatusInProgress, apisupport.StatusClosed},
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := repo.List(ctx, tc.statuses)
			require.NoError(t, err)
			assert.Len(t, items, tc.wantCount)
		})
	}
}

// 仕様 (FEATURE_SPEC §8.3): List は updated_at DESC 順で並べる。
// 直近 UPDATE された行が先頭に来ることを、index 単位のケースで検証する。
func TestInquiryList_仕様_並び順(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	// seed: 受付順 inq1 → inq2 → inq3。その後 inq2 を in_progress、inq3 を closed に更新。
	// 直近 UPDATE された順は inq3 → inq2 → inq1 になる。
	inq1, err := repo.Create(ctx, "first", "B", "u1@e.com")
	require.NoError(t, err)
	inq2, err := repo.Create(ctx, "second", "B", "u2@e.com")
	require.NoError(t, err)
	inq3, err := repo.Create(ctx, "third", "B", "u3@e.com")
	require.NoError(t, err)

	_, err = repo.UpdateStatus(ctx, inq2.InquiryID, apisupport.StatusInProgress)
	require.NoError(t, err)
	_, err = repo.UpdateStatus(ctx, inq3.InquiryID, apisupport.StatusClosed)
	require.NoError(t, err)

	items, err := repo.List(ctx, apisupport.Statuses)
	require.NoError(t, err)
	require.Len(t, items, 3)

	cases := []struct {
		name   string
		index  int
		wantID int64
	}{
		{
			name:   "index 0 は直近 UPDATE された inq3 (closed 遷移)",
			index:  0,
			wantID: inq3.InquiryID,
		},
		{
			name:   "index 1 は次の inq2 (in_progress 遷移)",
			index:  1,
			wantID: inq2.InquiryID,
		},
		{
			name:   "index 2 は受付のみで UPDATE されていない inq1",
			index:  2,
			wantID: inq1.InquiryID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantID, items[tc.index].InquiryID)
		})
	}
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
		status apisupport.Status
	}{
		{
			name:   "new",
			status: apisupport.StatusNew,
		},
		{
			name:   "in_progress",
			status: apisupport.StatusInProgress,
		},
		{
			name:   "closed",
			status: apisupport.StatusClosed,
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

	_, err = repo.UpdateStatus(ctx, inq.InquiryID, apisupport.Status("invalid"))
	assert.Error(t, err)
}

// 仕様 (FEATURE_SPEC §8.1): 未存在 ID への UpdateStatus は port.ErrNotFound を返す。
func TestUpdateStatus_仕様_未存在IDはErrNotFound(t *testing.T) {
	repo := newInquiryRepo(t)
	ctx := context.Background()

	_, err := repo.UpdateStatus(ctx, 999999, apisupport.StatusClosed)
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
