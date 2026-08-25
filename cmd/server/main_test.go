package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/config"
)

func TestSetupLogger(t *testing.T) {
	t.Run("[プロセス起動]ログ設定の構築", func(t *testing.T) {
		validEnvs := []struct {
			name string
			env  config.Env
		}{
			{"envがlocalのとき、エラーにならない", config.EnvLocal},
			{"envがstagingのとき、エラーにならない", config.EnvStaging},
			{"envがproductionのとき、エラーにならない", config.EnvProduction},
		}
		for _, tt := range validEnvs {
			t.Run(tt.name, func(t *testing.T) {
				assert.NoError(t, setupLogger(tt.env))
			})
		}

		t.Run("envがそれ以外の値invalidのとき、エラーになる", func(t *testing.T) {
			assert.Error(t, setupLogger(config.Env("invalid")))
		})
	})
}

func TestServe(t *testing.T) {
	t.Run("[プロセス起動]プロセスの待ち受けとgraceful shutdown", func(t *testing.T) {
		t.Run("指定したリスナーで待ち受けを開始し、設定したハンドラがリクエストに応答する", func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			srv := &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("serveテスト応答"))
				}),
			}
			ctx, cancel := context.WithCancel(context.Background())
			errCh := make(chan error, 1)
			go func() { errCh <- serve(ctx, srv, ln) }()
			t.Cleanup(func() {
				cancel()
				<-errCh
			})

			addr := "http://" + ln.Addr().String() + "/"
			var body []byte
			require.Eventually(t, func() bool {
				resp, getErr := http.Get(addr)
				if getErr != nil {
					return false
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return false
				}
				body, getErr = io.ReadAll(resp.Body)
				return getErr == nil
			}, 2*time.Second, 10*time.Millisecond)

			assert.Equal(t, "serveテスト応答", string(body))
		})

		t.Run("ctxがキャンセルされると、graceful shutdownを行いエラー無く終了する", func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			srv := &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			}
			ctx, cancel := context.WithCancel(context.Background())
			errCh := make(chan error, 1)
			go func() { errCh <- serve(ctx, srv, ln) }()

			require.Eventually(t, func() bool {
				conn, dialErr := net.Dial("tcp", ln.Addr().String())
				if dialErr != nil {
					return false
				}
				_ = conn.Close()
				return true
			}, 2*time.Second, 10*time.Millisecond)

			cancel()

			select {
			case serveErr := <-errCh:
				assert.NoError(t, serveErr)
			case <-time.After(2 * time.Second):
				t.Fatal("serveがctxキャンセル後も終了しない")
			}
		})
	})
}
