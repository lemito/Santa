package httpapi

import (
	"html/template"
	"io/fs"
	"path/filepath"
)

type Templates struct {
	FS fs.FS
	T  *template.Template
}

func LoadTemplates(dir string) (*Templates, error) {
	t, err := template.ParseGlob(filepath.Join(dir, "*.html"))
	if err != nil {
		return nil, err
	}
	if t, err = t.ParseGlob(filepath.Join(dir, "partials", "*.html")); err != nil {
		return nil, err
	}
	return &Templates{T: t}, nil
}
