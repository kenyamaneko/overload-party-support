package admin

import (
	"embed"
	"fmt"
	"html/template"
	"time"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

// templates は管理 UI が描画するテンプレートセット。
type templates struct {
	list *template.Template // layout + list + partials/row
	new  *template.Template // layout + new
	edit *template.Template // layout + edit
	row  *template.Template // partials/row 単体 (削除後の行消去などに使う)
}

// parseTemplates は embed.FS からテンプレート集合を組み立てる。起動時に 1 回だけ呼ぶ。
func parseTemplates() (*templates, error) {
	base := template.New("").Funcs(templateFuncMap())

	list, err := base.Clone()
	if err != nil {
		return nil, fmt.Errorf("clone base for list: %w", err)
	}
	list, err = list.ParseFS(templatesFS, "templates/layout.html", "templates/list.html", "templates/partials/row.html")
	if err != nil {
		return nil, fmt.Errorf("parse list templates: %w", err)
	}

	newTpl, err := base.Clone()
	if err != nil {
		return nil, fmt.Errorf("clone base for new: %w", err)
	}
	newTpl, err = newTpl.ParseFS(templatesFS, "templates/layout.html", "templates/new.html")
	if err != nil {
		return nil, fmt.Errorf("parse new templates: %w", err)
	}

	edit, err := base.Clone()
	if err != nil {
		return nil, fmt.Errorf("clone base for edit: %w", err)
	}
	edit, err = edit.ParseFS(templatesFS, "templates/layout.html", "templates/edit.html")
	if err != nil {
		return nil, fmt.Errorf("parse edit templates: %w", err)
	}

	row, err := base.Clone()
	if err != nil {
		return nil, fmt.Errorf("clone base for row: %w", err)
	}
	row, err = row.ParseFS(templatesFS, "templates/partials/row.html")
	if err != nil {
		return nil, fmt.Errorf("parse row template: %w", err)
	}

	return &templates{list: list, new: newTpl, edit: edit, row: row}, nil
}

// templateFuncMap は Go テンプレートから呼べるヘルパー関数を集める。
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		// findTranslation は指定 lang の翻訳を検索して返す。存在しなければ nil。
		"findTranslation": func(translations []apisupport.Translation, lang string) *apisupport.Translation {
			for i := range translations {
				if translations[i].Lang == lang {
					return &translations[i]
				}
			}
			return nil
		},
		// formatDatetimeLocal は *time.Time を input[type=datetime-local] の value に使う "YYYY-MM-DDTHH:MM" に整形。nil は空文字。
		"formatDatetimeLocal": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format("2006-01-02T15:04")
		},
	}
}
