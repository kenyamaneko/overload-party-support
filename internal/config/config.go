// Package config は環境変数からサービス起動に必要な設定を読み込む。
package config

import (
	"fmt"
	"os"
	"strconv"
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

	// PostgreSQL 接続文字列 (pgx 形式)。
	DatabaseConn string

	// Cloud SQL への接続認証方式を切り替えるフラグ。
	DatabaseIAMAuthEnabled bool

	// IAM データベース認証を使うときのみ必須。
	CloudSQLConnectionName string
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

	databaseConn, err := getRequiredString("DATABASE_CONN")
	if err != nil {
		return nil, err
	}

	databaseIAMAuthEnabled, err := getRequiredBool("DATABASE_IAM_AUTH_ENABLED")
	if err != nil {
		return nil, err
	}
	var cloudSQLConnectionName string
	if databaseIAMAuthEnabled {
		cloudSQLConnectionName = os.Getenv("CLOUDSQL_CONNECTION_NAME")
		if cloudSQLConnectionName == "" {
			return nil, fmt.Errorf("CLOUDSQL_CONNECTION_NAME is required when DATABASE_IAM_AUTH_ENABLED=true")
		}
	}

	return &Config{
		Env:                    env,
		InternalPort:           internalPort,
		DatabaseConn:           databaseConn,
		DatabaseIAMAuthEnabled: databaseIAMAuthEnabled,
		CloudSQLConnectionName: cloudSQLConnectionName,
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

// getRequiredString は必須の文字列 env を取得する。空文字列なら error。
func getRequiredString(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// getRequiredBool は必須の真偽 env を取得する。"true" / "false" 以外はエラー。
func getRequiredBool(name string) (bool, error) {
	v := os.Getenv(name)
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be %q or %q, got %q", name, "true", "false", v)
	}
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
