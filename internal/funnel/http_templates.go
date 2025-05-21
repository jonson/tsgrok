package funnel

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"

	"github.com/jonson/tsgrok/web"
)

func loadTemplates() (*template.Template, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"default": func(defaultValue interface{}, givenValue interface{}) interface{} {
			// Check for zero values for common types
			gv := reflect.ValueOf(givenValue)
			if !gv.IsValid() || gv.IsZero() {
				return defaultValue
			}
			// Special case for strings, as an empty string is a zero value but might be intended
			// However, for typical "default" usage, empty string means use default.
			if gv.Kind() == reflect.String && gv.String() == "" {
				return defaultValue
			}
			return givenValue
		},
	}).ParseFS(web.TemplatesFS, "templates/*.html")

	if err != nil {
		return nil, fmt.Errorf("failed to parse template(s) from embedded fs: %w", err)
	}
	return tmpl, nil
}
