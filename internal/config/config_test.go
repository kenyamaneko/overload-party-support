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

// 仕様 (CLAUDE.md): デフォルト値フォールバックなし。env の妥当性を即 fail する。
func TestFromEnv_RequiredAndValidation(t *testing.T) {
	cases := []struct {
		name    string
		base    func() map[string]string
		mutate  func(m map[string]string)
		wantErr bool
	}{
		{
			name:    "local 正常",
			base:    validLocalEnv,
			mutate:  func(_ map[string]string) {},
			wantErr: false,
		},
		{
			name:    "staging (token 揃い) 正常",
			base:    validProdEnv,
			mutate:  func(m map[string]string) { m["ENV"] = "staging" },
			wantErr: false,
		},
		{
			name:    "production (token 揃い) 正常",
			base:    validProdEnv,
			mutate:  func(_ map[string]string) {},
			wantErr: false,
		},

		{
			name:    "ENV 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "ENV") },
			wantErr: true,
		},
		{
			name:    "ENV 未知値",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["ENV"] = "dev" },
			wantErr: true,
		},

		{
			name:    "INTERNAL_PORT 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "INTERNAL_PORT") },
			wantErr: true,
		},
		{
			name:    "INTERNAL_PORT 非整数",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["INTERNAL_PORT"] = "abc" },
			wantErr: true,
		},
		{
			name:    "ADMIN_PORT 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "ADMIN_PORT") },
			wantErr: true,
		},
		{
			name:    "EXTERNAL_PORT 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "EXTERNAL_PORT") },
			wantErr: true,
		},
		{
			name:    "internal と admin が同ポート",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["ADMIN_PORT"] = m["INTERNAL_PORT"] },
			wantErr: true,
		},
		{
			name:    "internal と external が同ポート",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["EXTERNAL_PORT"] = m["INTERNAL_PORT"] },
			wantErr: true,
		},
		{
			name:    "admin と external が同ポート",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["EXTERNAL_PORT"] = m["ADMIN_PORT"] },
			wantErr: true,
		},

		{
			name:    "DATABASE_CONN 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "DATABASE_CONN") },
			wantErr: true,
		},

		{
			name:    "CORS_ALLOWED_ORIGINS 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "CORS_ALLOWED_ORIGINS") },
			wantErr: true,
		},
		{
			name:    "CORS_ALLOWED_ORIGINS 空 (カンマのみ)",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["CORS_ALLOWED_ORIGINS"] = ", ," },
			wantErr: true,
		},

		{
			name:    "INQUIRY_BODY_SNIPPET_LENGTH 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "INQUIRY_BODY_SNIPPET_LENGTH") },
			wantErr: true,
		},
		{
			name:    "INQUIRY_BODY_SNIPPET_LENGTH 0",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["INQUIRY_BODY_SNIPPET_LENGTH"] = "0" },
			wantErr: true,
		},
		{
			name:    "INQUIRY_BODY_SNIPPET_LENGTH 負値",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { m["INQUIRY_BODY_SNIPPET_LENGTH"] = "-1" },
			wantErr: true,
		},

		{
			name:    "SENDGRID_FROM_ADDRESS 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "SENDGRID_FROM_ADDRESS") },
			wantErr: true,
		},
		{
			name:    "SENDGRID_FROM_NAME 欠け",
			base:    validLocalEnv,
			mutate:  func(m map[string]string) { delete(m, "SENDGRID_FROM_NAME") },
			wantErr: true,
		},

		{
			name:    "production で SLACK_BOT_TOKEN 欠け",
			base:    validProdEnv,
			mutate:  func(m map[string]string) { delete(m, "SLACK_BOT_TOKEN") },
			wantErr: true,
		},
		{
			name:    "production で SLACK_CHANNEL_ID 欠け",
			base:    validProdEnv,
			mutate:  func(m map[string]string) { delete(m, "SLACK_CHANNEL_ID") },
			wantErr: true,
		},
		{
			name:    "production で SENDGRID_API_KEY 欠け",
			base:    validProdEnv,
			mutate:  func(m map[string]string) { delete(m, "SENDGRID_API_KEY") },
			wantErr: true,
		},
		{
			name: "staging で SLACK_BOT_TOKEN 欠け",
			base: validProdEnv,
			mutate: func(m map[string]string) {
				m["ENV"] = "staging"
				delete(m, "SLACK_BOT_TOKEN")
			},
			wantErr: true,
		},

		{
			name: "local は SLACK_BOT_TOKEN 無くても可",
			base: validLocalEnv,
			mutate: func(m map[string]string) {
				delete(m, "SLACK_BOT_TOKEN")
				delete(m, "SLACK_CHANNEL_ID")
				delete(m, "SENDGRID_API_KEY")
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := tc.base()
			tc.mutate(m)
			applyEnv(t, m)

			cfg, err := config.FromEnv()

			assertErrExpectation(t, err, tc.wantErr)
			assert.Equal(t, tc.wantErr, cfg == nil)
		})
	}
}

// 仕様: 正常な env から生成された Config はすべてのフィールドが env の値を反映する。
func TestFromEnv_ReflectsAllEnv(t *testing.T) {
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

func assertErrExpectation(t *testing.T, err error, wantErr bool) {
	t.Helper()
	assert.Equal(t, wantErr, err != nil, "err expectation: wantErr=%v, got=%v", wantErr, err)
}
