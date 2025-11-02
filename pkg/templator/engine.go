package templator

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type Engine struct {
	templates map[string]*template.Template
}

func NewEngine() *Engine {
	return &Engine{
		templates: make(map[string]*template.Template),
	}
}

func (e *Engine) LoadTemplate(name, path string) error {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return fmt.Errorf("failed to load template %s from %s: %w", name, path, err)
	}
	e.templates[name] = tmpl
	return nil
}

func (e *Engine) HasTemplate(name string) bool {
	_, exists := e.templates[name]
	return exists
}

func (e *Engine) RenderToFile(name, outputPath string, data any) error {
	tmpl, exists := e.templates[name]
	if !exists {
		return fmt.Errorf("template %s not found", name)
	}

	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to render template %s: %w", name, err)
	}

	return nil
}

func (e *Engine) RenderToBytes(name string, data any) ([]byte, error) {
	tmpl, exists := e.templates[name]
	if !exists {
		return nil, fmt.Errorf("template %s not found", name)
	}

	buf := bytes.NewBuffer([]byte{})
	if err := tmpl.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("failed to render template %s: %w", name, err)
	}

	return buf.Bytes(), nil
}
