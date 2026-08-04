package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/config"
)

// envKeys は Config が参照する全 env 変数。テスト間の残留を潰すために使う。
var envKeys = []string{
	"ENV",
	"INTERNAL_PORT",
	"DATABASE_CONN",
	"DATABASE_IAM_AUTH_ENABLED",
	"CLOUDSQL_CONNECTION_NAME",
}

// validLocalEnv は ENV=local で正常に起動する最小セット。
func validLocalEnv() map[string]string {
	return map[string]string{
		"ENV":                       "local",
		"INTERNAL_PORT":             "9009",
		"DATABASE_CONN":             "host=localhost dbname=support",
		"DATABASE_IAM_AUTH_ENABLED": "false",
	}
}

// validProdEnv は ENV=production 下での最小セット。
func validProdEnv() map[string]string {
	m := validLocalEnv()
	m["ENV"] = "production"
	return m
}

func TestFromEnv(t *testing.T) {
	t.Run("環境変数からの設定構築", func(t *testing.T) {
		validCases := []struct {
			name   string
			base   func() map[string]string
			mutate func(m map[string]string)
		}{
			{
				name:   "local の必須 env が揃うとき、Config が構築される",
				base:   validLocalEnv,
				mutate: func(_ map[string]string) {},
			},
			{
				name:   "ENV が staging のとき、Config が構築される",
				base:   validProdEnv,
				mutate: func(m map[string]string) { m["ENV"] = "staging" },
			},
			{
				name:   "ENV が production のとき、Config が構築される",
				base:   validProdEnv,
				mutate: func(_ map[string]string) {},
			},
		}
		for _, tc := range validCases {
			t.Run(tc.name, func(t *testing.T) {
				m := tc.base()
				tc.mutate(m)
				applyEnv(t, m)

				cfg, err := config.FromEnv()

				require.NoError(t, err)
				require.NotNil(t, cfg)
			})
		}

		// 欠落・不正な env はデフォルト値へフォールバックせず即エラーにする。
		invalidCases := []struct {
			name            string
			base            func() map[string]string
			mutate          func(m map[string]string)
			wantErrContains string
		}{
			{
				name:            "ENV が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "ENV") },
				wantErrContains: "ENV is required",
			},
			{
				name:            "ENV が未知値のとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["ENV"] = "dev" },
				wantErrContains: "ENV: unsupported value",
			},
			{
				name:            "INTERNAL_PORT が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "INTERNAL_PORT") },
				wantErrContains: "INTERNAL_PORT is required",
			},
			{
				name:            "INTERNAL_PORT が非整数のとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["INTERNAL_PORT"] = "abc" },
				wantErrContains: "INTERNAL_PORT: not an integer",
			},
			{
				name:            "DATABASE_CONN が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "DATABASE_CONN") },
				wantErrContains: "DATABASE_CONN is required",
			},
			{
				name:            "DATABASE_IAM_AUTH_ENABLED が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "DATABASE_IAM_AUTH_ENABLED") },
				wantErrContains: "DATABASE_IAM_AUTH_ENABLED must be",
			},
			{
				name:            `DATABASE_IAM_AUTH_ENABLED が "true"/"false" 以外の "yes" のとき、エラーになる`,
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["DATABASE_IAM_AUTH_ENABLED"] = "yes" },
				wantErrContains: "DATABASE_IAM_AUTH_ENABLED must be",
			},
			{
				name: "DATABASE_IAM_AUTH_ENABLED が true かつ CLOUDSQL_CONNECTION_NAME が未設定のとき、エラーになる",
				base: validLocalEnv,
				mutate: func(m map[string]string) {
					m["DATABASE_IAM_AUTH_ENABLED"] = "true"
				},
				wantErrContains: "CLOUDSQL_CONNECTION_NAME is required",
			},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				m := tc.base()
				tc.mutate(m)
				applyEnv(t, m)

				cfg, err := config.FromEnv()

				require.Error(t, err)
				require.Nil(t, cfg)
				assert.Contains(t, err.Error(), tc.wantErrContains)
			})
		}

		t.Run("全ての環境変数が設定値に反映される", func(t *testing.T) {
			m := validProdEnv()
			m["INTERNAL_PORT"] = "12345"
			applyEnv(t, m)

			cfg, err := config.FromEnv()

			require.NoError(t, err)
			assert.Equal(t, config.EnvProduction, cfg.Env)
			assert.Equal(t, 12345, cfg.InternalPort)
			assert.Equal(t, m["DATABASE_CONN"], cfg.DatabaseConn)
			assert.False(t, cfg.DatabaseIAMAuthEnabled)
			assert.Empty(t, cfg.CloudSQLConnectionName)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLED が true のとき、CLOUDSQL_CONNECTION_NAME が Config に反映される", func(t *testing.T) {
			m := validLocalEnv()
			m["DATABASE_IAM_AUTH_ENABLED"] = "true"
			m["CLOUDSQL_CONNECTION_NAME"] = "overload-party-dev:asia-northeast1:overload-party-db"
			applyEnv(t, m)

			cfg, err := config.FromEnv()

			require.NoError(t, err)
			assert.True(t, cfg.DatabaseIAMAuthEnabled)
			assert.Equal(t, m["CLOUDSQL_CONNECTION_NAME"], cfg.CloudSQLConnectionName)
		})
	})
}

// applyEnv は対象 env をいったん空にしてから m の値を設定する (t.Setenv は自動 cleanup)。
func applyEnv(t *testing.T, m map[string]string) {
	t.Helper()
	for _, k := range envKeys {
		t.Setenv(k, "")
	}
	for k, v := range m {
		t.Setenv(k, v)
	}
}
