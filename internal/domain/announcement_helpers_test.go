package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

func TestIsSupportedLang(t *testing.T) {
	t.Run("[お知らせドメインモデル]対応言語の判定", func(t *testing.T) {
		tests := []struct {
			name string
			lang string
			want bool
		}{
			{"langがjaのとき、対応言語と判定される", "ja", true},
			{"langがenのとき、対応言語と判定される", "en", true},
			{"langが対応言語一覧に無い値frのとき、対応言語でないと判定される", "fr", false},
			{"langが空文字のとき、対応言語でないと判定される", "", false},
			{"langの大文字小文字が対応言語と異なる値JAのとき、対応言語でないと判定される", "JA", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, domain.IsSupportedLang(tt.lang))
			})
		}
	})
}
