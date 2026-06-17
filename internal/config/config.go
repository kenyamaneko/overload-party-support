// Package config は環境変数からサービス起動に必要な設定を読み込む。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Env はサービスの動作環境を表す enum。
type Env string

const (
	EnvLocal      Env = "local"
	EnvStaging    Env = "staging"
	EnvProduction Env = "production"
)

// Config はサービス全体の起動時設定。
type Config struct {
	Env Env

	// HTTP ポート。
	InternalPort int
	AdminPort    int
	ExternalPort int

	// PostgreSQL 接続文字列 (pgx 形式)。
	DatabaseConn string

	// 外部フォーム用 CORS 許可オリジン。
	CORSAllowedOrigins []string

	// Slack 通知用本文抜粋の文字数。
	InquiryBodySnippetLength int

	// SendGrid 共通設定。
	SendGridFromAddress string
	SendGridFromName    string

	// Env != local 時のみ必須。
	SlackBotToken  string
	SlackChannelID string
	SendGridAPIKey string
}

// FromEnv は process env から Config を構築する。必須変数が未設定の場合はエラー。
func FromEnv() (*Config, error) {
	env, err := parseEnv(os.Getenv("ENV"))
	if err != nil {
		return nil, err
	}

	internalPort, err := getRequiredInt("INTERNAL_PORT")
	if err != nil {
		return nil, err
	}
	adminPort, err := getRequiredInt("ADMIN_PORT")
	if err != nil {
		return nil, err
	}
	externalPort, err := getRequiredInt("EXTERNAL_PORT")
	if err != nil {
		return nil, err
	}
	if err := validatePortsDistinct(internalPort, adminPort, externalPort); err != nil {
		return nil, err
	}

	databaseConn, err := getRequiredString("DATABASE_CONN")
	if err != nil {
		return nil, err
	}

	originsRaw, err := getRequiredString("CORS_ALLOWED_ORIGINS")
	if err != nil {
		return nil, err
	}
	origins := splitAndTrim(originsRaw)
	if len(origins) == 0 {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS: at least one origin required")
	}

	snippetLen, err := getRequiredInt("INQUIRY_BODY_SNIPPET_LENGTH")
	if err != nil {
		return nil, err
	}
	if snippetLen <= 0 {
		return nil, fmt.Errorf("INQUIRY_BODY_SNIPPET_LENGTH: must be positive, got %d", snippetLen)
	}

	fromAddr, err := getRequiredString("SENDGRID_FROM_ADDRESS")
	if err != nil {
		return nil, err
	}
	fromName, err := getRequiredString("SENDGRID_FROM_NAME")
	if err != nil {
		return nil, err
	}

	// Slack / SendGrid API token は prod/staging でのみ必須。
	// local は mock アダプタを使うので空でも OK。
	slackToken := os.Getenv("SLACK_BOT_TOKEN")
	slackChannel := os.Getenv("SLACK_CHANNEL_ID")
	sendGridKey := os.Getenv("SENDGRID_API_KEY")

	if env != EnvLocal {
		if slackToken == "" {
			return nil, fmt.Errorf("SLACK_BOT_TOKEN is required when ENV=%s", env)
		}
		if slackChannel == "" {
			return nil, fmt.Errorf("SLACK_CHANNEL_ID is required when ENV=%s", env)
		}
		if sendGridKey == "" {
			return nil, fmt.Errorf("SENDGRID_API_KEY is required when ENV=%s", env)
		}
	}

	return &Config{
		Env:                      env,
		InternalPort:             internalPort,
		AdminPort:                adminPort,
		ExternalPort:             externalPort,
		DatabaseConn:             databaseConn,
		CORSAllowedOrigins:       origins,
		InquiryBodySnippetLength: snippetLen,
		SendGridFromAddress:      fromAddr,
		SendGridFromName:         fromName,
		SlackBotToken:            slackToken,
		SlackChannelID:           slackChannel,
		SendGridAPIKey:           sendGridKey,
	}, nil
}

// parseEnv は ENV 文字列を Env enum に変換する。未定義値はエラー。
func parseEnv(s string) (Env, error) {
	switch Env(s) {
	case EnvLocal, EnvStaging, EnvProduction:
		return Env(s), nil
	case "":
		return "", fmt.Errorf("ENV is required")
	default:
		return "", fmt.Errorf("ENV: unsupported value %q", s)
	}
}

// validatePortsDistinct は 3 ポートがすべて異なることを確認する。
func validatePortsDistinct(internal, admin, external int) error {
	if internal == admin || internal == external || admin == external {
		return fmt.Errorf("INTERNAL_PORT / ADMIN_PORT / EXTERNAL_PORT must all differ (got %d / %d / %d)",
			internal, admin, external)
	}
	return nil
}

// getRequiredString は必須の文字列 env を取得する。空文字列なら error。
func getRequiredString(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// getRequiredInt は必須の整数 env を取得する。未設定 / 非整数ならエラー。
func getRequiredInt(name string) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: not an integer: %q", name, v)
	}
	return n, nil
}

// splitAndTrim はカンマ区切り文字列を trim 済み slice に分解する (空要素は除外)。
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
