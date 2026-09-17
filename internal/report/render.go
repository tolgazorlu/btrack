package report

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"
)

//go:embed assets/report.css assets/report.html.tmpl
var assets embed.FS

type renderData struct {
	Report
	CSS template.CSS
}

var funcs = template.FuncMap{
	"duration": FormatDuration,
	"join": func(items []string, sep string) string {
		return strings.Join(items, sep)
	},
	"thousands": func(n int) string {
		s := fmt.Sprintf("%d", n)
		if len(s) <= 3 {
			return s
		}
		var out []byte
		for i, c := range []byte(s) {
			if i > 0 && (len(s)-i)%3 == 0 {
				out = append(out, '.')
			}
			out = append(out, c)
		}
		return string(out)
	},
}

// Render writes the report as a self-contained HTML document.
func (r Report) Render(w io.Writer) error {
	css, err := assets.ReadFile("assets/report.css")
	if err != nil {
		return fmt.Errorf("read stylesheet: %w", err)
	}
	tmplSrc, err := assets.ReadFile("assets/report.html.tmpl")
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	tmpl, err := template.New("report").Funcs(funcs).Parse(string(tmplSrc))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	if r.GeneratedAt.IsZero() {
		r.GeneratedAt = time.Now()
	}
	if r.Title == "" {
		r.Title = r.Client + " Geliştirme Raporu"
	}
	return tmpl.Execute(w, renderData{Report: r, CSS: template.CSS(css)})
}
