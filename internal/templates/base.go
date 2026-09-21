// Package templates parses and renders the html/template set: a shared base
// of layout, components and partials, plus one clone per full page.
package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
)

// Pages lists the full-page templates under templates/. Each defines
// "content", which the "base" layout renders.
var Pages = []string{"index", "result"}

// Renderer executes page and partial templates.
type Renderer struct {
	shared *template.Template
	pages  map[string]*template.Template
}

// New parses templates from fsys (which must contain a templates/ directory).
func New(fsys fs.FS, funcs template.FuncMap) (*Renderer, error) {
	shared, err := template.New("").Funcs(funcs).ParseFS(fsys,
		"templates/base.html",
		"templates/components/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("templates: parse shared: %w", err)
	}

	r := &Renderer{shared: shared, pages: make(map[string]*template.Template, len(Pages))}
	for _, page := range Pages {
		// Clone before any execution: html/template forbids cloning afterwards.
		clone, err := shared.Clone()
		if err != nil {
			return nil, fmt.Errorf("templates: clone for %s: %w", page, err)
		}
		if _, err := clone.ParseFS(fsys, "templates/"+page+".html"); err != nil {
			return nil, fmt.Errorf("templates: parse %s: %w", page, err)
		}
		r.pages[page] = clone
	}
	return r, nil
}

// Page renders a full page through the base layout. Output is buffered so a
// template error never produces a half-written response.
func (r *Renderer) Page(w io.Writer, page string, data any) error {
	t, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("templates: unknown page %q", page)
	}
	return execute(w, t, "base", data)
}

// Partial renders a named template fragment such as "partials/form-success".
func (r *Renderer) Partial(w io.Writer, name string, data any) error {
	return execute(w, r.shared, name, data)
}

func execute(w io.Writer, t *template.Template, name string, data any) error {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("templates: execute %s: %w", name, err)
	}
	_, err := buf.WriteTo(w)
	return err
}
