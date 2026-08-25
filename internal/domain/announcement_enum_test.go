package domain_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

type openapiSpec struct {
	Components struct {
		Schemas struct {
			AnnouncementType struct {
				Enum []string `yaml:"enum"`
			} `yaml:"AnnouncementType"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

func readOpenAPIAnnouncementTypeEnum(t *testing.T) []string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this test file's path")
	openapiPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "data", "openapi.yaml")

	data, err := os.ReadFile(openapiPath)
	require.NoError(t, err)

	var spec openapiSpec
	require.NoError(t, yaml.Unmarshal(data, &spec))

	return spec.Components.Schemas.AnnouncementType.Enum
}

func TestAnnouncementTypeOpenAPIContract(t *testing.T) {
	t.Run("[お知らせドメインモデル]AnnouncementTypeのdomain-openapi.yaml契約整合", func(t *testing.T) {
		t.Run("domainのお知らせ種別定数の集合が、info・maintenance・event・updateの集合と一致する", func(t *testing.T) {
			types := []string{domain.TypeInfo, domain.TypeMaintenance, domain.TypeEvent, domain.TypeUpdate}
			assert.ElementsMatch(t, []string{"info", "maintenance", "event", "update"}, types)
		})

		t.Run("openapi.yamlのAnnouncementType.enumの値集合が、info・maintenance・event・updateの集合と一致する", func(t *testing.T) {
			enum := readOpenAPIAnnouncementTypeEnum(t)
			assert.ElementsMatch(t, []string{"info", "maintenance", "event", "update"}, enum)
		})
	})
}
