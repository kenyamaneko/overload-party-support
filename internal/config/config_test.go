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
	"ADMIN_PORT",
	"EXTERNAL_PORT",
	"DATABASE_CONN",
	"CORS_ALLOWED_ORIGINS",
	"INQUIRY_BODY_SNIPPET_LENGTH",
	"SENDGRID_FROM_ADDRESS",
	"SENDGRID_FROM_NAME",
	"SLACK_BOT_TOKEN",
	"SLACK_CHANNEL_ID",
	"SENDGRID_API_KEY",
}

// validLocalEnv は ENV=local (Slack/SendGrid token 無し) で正常に起動する最小セット。
func validLocalEnv() map[string]string {
	return map[string]string{
		"ENV":                         "local",
		"INTERNAL_PORT":               "9009",
		"ADMIN_PORT":                  "9109",
		"EXTERNAL_PORT":               "9209",
		"DATABASE_CONN":               "host=localhost dbname=support",
		"CORS_ALLOWED_ORIGINS":        "http://localhost:3000",
		"INQUIRY_BODY_SNIPPET_LENGTH": "200",
		"SENDGRID_FROM_ADDRESS":       "support@example.com",
		"SENDGRID_FROM_NAME":          "Overload Party Support",
	}
}

// validProdEnv は ENV=production 下での最小セット (Slack/SendGrid token 必須)。
func validProdEnv() map[string]string {
	m := validLocalEnv()
	m["ENV"] = "production"
	m["SLACK_BOT_TOKEN"] = "xoxb-token"
	m["SLACK_CHANNEL_ID"] = "C12345"
	m["SENDGRID_API_KEY"] = "SG.key"
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
				name:   "ENV が staging で token が揃うとき、Config が構築される",
				base:   validProdEnv,
				mutate: func(m map[string]string) { m["ENV"] = "staging" },
			},
			{
				name:   "ENV が production で token が揃うとき、Config が構築される",
				base:   validProdEnv,
				mutate: func(_ map[string]string) {},
			},
			{
				name: "local で SLACK/SENDGRID token が無くても、Config が構築される",
				base: validLocalEnv,
				mutate: func(m map[string]string) {
					delete(m, "SLACK_BOT_TOKEN")
					delete(m, "SLACK_CHANNEL_ID")
					delete(m, "SENDGRID_API_KEY")
				},
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
				name:            "ADMIN_PORT が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "ADMIN_PORT") },
				wantErrContains: "ADMIN_PORT is required",
			},
			{
				name:            "EXTERNAL_PORT が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "EXTERNAL_PORT") },
				wantErrContains: "EXTERNAL_PORT is required",
			},
			{
				name:            "internal と admin が同ポートのとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["ADMIN_PORT"] = m["INTERNAL_PORT"] },
				wantErrContains: "must all differ",
			},
			{
				name:            "internal と external が同ポートのとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["EXTERNAL_PORT"] = m["INTERNAL_PORT"] },
				wantErrContains: "must all differ",
			},
			{
				name:            "admin と external が同ポートのとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["EXTERNAL_PORT"] = m["ADMIN_PORT"] },
				wantErrContains: "must all differ",
			},
			{
				name:            "DATABASE_CONN が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "DATABASE_CONN") },
				wantErrContains: "DATABASE_CONN is required",
			},
			{
				name:            "CORS_ALLOWED_ORIGINS が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "CORS_ALLOWED_ORIGINS") },
				wantErrContains: "CORS_ALLOWED_ORIGINS is required",
			},
			{
				name:            "CORS_ALLOWED_ORIGINS が空 (カンマのみ) のとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["CORS_ALLOWED_ORIGINS"] = ", ," },
				wantErrContains: "at least one origin required",
			},
			{
				name:            "INQUIRY_BODY_SNIPPET_LENGTH が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "INQUIRY_BODY_SNIPPET_LENGTH") },
				wantErrContains: "INQUIRY_BODY_SNIPPET_LENGTH is required",
			},
			{
				name:            "INQUIRY_BODY_SNIPPET_LENGTH が 0 のとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["INQUIRY_BODY_SNIPPET_LENGTH"] = "0" },
				wantErrContains: "must be positive, got 0",
			},
			{
				name:            "INQUIRY_BODY_SNIPPET_LENGTH が -1 のとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { m["INQUIRY_BODY_SNIPPET_LENGTH"] = "-1" },
				wantErrContains: "must be positive, got -1",
			},
			{
				name:            "SENDGRID_FROM_ADDRESS が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "SENDGRID_FROM_ADDRESS") },
				wantErrContains: "SENDGRID_FROM_ADDRESS is required",
			},
			{
				name:            "SENDGRID_FROM_NAME が欠けるとき、エラーになる",
				base:            validLocalEnv,
				mutate:          func(m map[string]string) { delete(m, "SENDGRID_FROM_NAME") },
				wantErrContains: "SENDGRID_FROM_NAME is required",
			},
			{
				name:            "production で SLACK_BOT_TOKEN が欠けるとき、エラーになる",
				base:            validProdEnv,
				mutate:          func(m map[string]string) { delete(m, "SLACK_BOT_TOKEN") },
				wantErrContains: "SLACK_BOT_TOKEN is required when ENV=production",
			},
			{
				name:            "production で SLACK_CHANNEL_ID が欠けるとき、エラーになる",
				base:            validProdEnv,
				mutate:          func(m map[string]string) { delete(m, "SLACK_CHANNEL_ID") },
				wantErrContains: "SLACK_CHANNEL_ID is required when ENV=production",
			},
			{
				name:            "production で SENDGRID_API_KEY が欠けるとき、エラーになる",
				base:            validProdEnv,
				mutate:          func(m map[string]string) { delete(m, "SENDGRID_API_KEY") },
				wantErrContains: "SENDGRID_API_KEY is required when ENV=production",
			},
			{
				name: "staging で SLACK_BOT_TOKEN が欠けるとき、エラーになる",
				base: validProdEnv,
				mutate: func(m map[string]string) {
					m["ENV"] = "staging"
					delete(m, "SLACK_BOT_TOKEN")
				},
				wantErrContains: "SLACK_BOT_TOKEN is required when ENV=staging",
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
			m["ADMIN_PORT"] = "12346"
			m["EXTERNAL_PORT"] = "12347"
			m["CORS_ALLOWED_ORIGINS"] = "https://a.example.com, https://b.example.com"
			m["INQUIRY_BODY_SNIPPET_LENGTH"] = "150"
			applyEnv(t, m)

			cfg, err := config.FromEnv()

			require.NoError(t, err)
			assert.Equal(t, config.EnvProduction, cfg.Env)
			assert.Equal(t, 12345, cfg.InternalPort)
			assert.Equal(t, 12346, cfg.AdminPort)
			assert.Equal(t, 12347, cfg.ExternalPort)
			assert.Equal(t, m["DATABASE_CONN"], cfg.DatabaseConn)
			assert.Equal(t, []string{"https://a.example.com", "https://b.example.com"}, cfg.CORSAllowedOrigins)
			assert.Equal(t, 150, cfg.InquiryBodySnippetLength)
			assert.Equal(t, m["SENDGRID_FROM_ADDRESS"], cfg.SendGridFromAddress)
			assert.Equal(t, m["SENDGRID_FROM_NAME"], cfg.SendGridFromName)
			assert.Equal(t, m["SLACK_BOT_TOKEN"], cfg.SlackBotToken)
			assert.Equal(t, m["SLACK_CHANNEL_ID"], cfg.SlackChannelID)
			assert.Equal(t, m["SENDGRID_API_KEY"], cfg.SendGridAPIKey)
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
