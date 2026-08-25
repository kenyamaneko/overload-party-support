package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/config"
)

// unsetEnv は key を未設定にし、テスト終了時に元の値へ戻す。
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, hadValue := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if hadValue {
			require.NoError(t, os.Setenv(key, orig))
			return
		}
		require.NoError(t, os.Unsetenv(key))
	})
}

// setBaselineEnv は FromEnv が成功する最小限の環境変数一式を設定する。
func setBaselineEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "local")
	t.Setenv("INTERNAL_PORT", "8080")
	t.Setenv("DATABASE_CONN", "postgres://test:test@localhost:5432/test")
	t.Setenv("DATABASE_IAM_AUTH_ENABLED", "false")
}

func TestFromEnv(t *testing.T) {
	t.Run("[起動設定]起動設定の構築", func(t *testing.T) {
		type testCase struct {
			name    string
			arrange func(t *testing.T)
			verify  func(t *testing.T, cfg *config.Config, err error)
		}

		tests := []testCase{
			{
				name: "ENVがlocalのとき、設定が構築され、Envフィールドにlocalが反映される",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("ENV", "local")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.NoError(t, err)
					require.Equal(t, config.EnvLocal, cfg.Env)
				},
			},
			{
				name: "ENVがstagingのとき、設定が構築され、Envフィールドにstagingが反映される",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("ENV", "staging")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.NoError(t, err)
					require.Equal(t, config.EnvStaging, cfg.Env)
				},
			},
			{
				name: "ENVがproductionのとき、設定が構築され、Envフィールドにproductionが反映される",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("ENV", "production")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.NoError(t, err)
					require.Equal(t, config.EnvProduction, cfg.Env)
				},
			},
			{
				name: "ENVが未設定のとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					unsetEnv(t, "ENV")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "ENVがlocal・staging・productionのいずれでもない値xyzのとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("ENV", "xyz")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "INTERNAL_PORTが未設定のとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					unsetEnv(t, "INTERNAL_PORT")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "INTERNAL_PORTが整数でない値abcのとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("INTERNAL_PORT", "abc")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "DATABASE_CONNが未設定のとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					unsetEnv(t, "DATABASE_CONN")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "DATABASE_IAM_AUTH_ENABLEDが未設定のとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					unsetEnv(t, "DATABASE_IAM_AUTH_ENABLED")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "DATABASE_IAM_AUTH_ENABLEDがtrue・falseのいずれでもない値yesのとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("DATABASE_IAM_AUTH_ENABLED", "yes")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "DATABASE_IAM_AUTH_ENABLEDがtrueかつCLOUDSQL_CONNECTION_NAMEが未設定のとき、エラーになる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("DATABASE_IAM_AUTH_ENABLED", "true")
					unsetEnv(t, "CLOUDSQL_CONNECTION_NAME")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.Error(t, err)
				},
			},
			{
				name: "DATABASE_IAM_AUTH_ENABLEDがtrueかつCLOUDSQL_CONNECTION_NAMEが設定されているとき、設定が構築され、CLOUDSQL_CONNECTION_NAMEの値が反映される",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("DATABASE_IAM_AUTH_ENABLED", "true")
					t.Setenv("CLOUDSQL_CONNECTION_NAME", "test-project:test-region:test-instance")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.NoError(t, err)
					require.Equal(t, "test-project:test-region:test-instance", cfg.CloudSQLConnectionName)
				},
			},
			{
				name: "DATABASE_IAM_AUTH_ENABLEDがfalseのとき、構築された設定のCLOUDSQL_CONNECTION_NAMEは空になる",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("DATABASE_IAM_AUTH_ENABLED", "false")
					unsetEnv(t, "CLOUDSQL_CONNECTION_NAME")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.NoError(t, err)
					require.Empty(t, cfg.CloudSQLConnectionName)
				},
			},
			{
				name: "INTERNAL_PORTとDATABASE_CONNに設定した値が、構築された設定の対応するフィールドにそのまま反映される",
				arrange: func(t *testing.T) {
					setBaselineEnv(t)
					t.Setenv("INTERNAL_PORT", "9123")
					t.Setenv("DATABASE_CONN", "postgres://custom:custom@example.invalid:5432/custom")
				},
				verify: func(t *testing.T, cfg *config.Config, err error) {
					require.NoError(t, err)
					require.Equal(t, 9123, cfg.InternalPort)
					require.Equal(t, "postgres://custom:custom@example.invalid:5432/custom", cfg.DatabaseConn)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.arrange(t)
				cfg, err := config.FromEnv()
				tt.verify(t, cfg, err)
			})
		}
	})
}
