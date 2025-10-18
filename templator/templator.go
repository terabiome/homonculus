package templator

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/template"
)

type TemplateName string
type TemplateHandler func() (*template.Template, error)

var templateMap = map[TemplateName]TemplateHandler{
	VIRTUAL_MACHINE: loadVirtualMachineTemplate,
}

type Templator struct {
	tmpl *template.Template
}

func New(templateName TemplateName) (*Templator, error) {
	handler, handlerExist := templateMap[templateName]
	if !handlerExist {
		return nil, errors.New("could not create templator: no matching template")
	}

	tmpl, err := handler()
	if err != nil {
		return nil, fmt.Errorf("could not create templator: %v", err)
	}

	return &Templator{tmpl}, nil
}

func (t *Templator) ToWriter(wr io.Writer, data any) error {
	return t.tmpl.Execute(wr, data)
}

func (t *Templator) ToFile(filepath string, data any) error {
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE, 0o777)
	if err != nil {
		return fmt.Errorf("could not write to file: %v", err)
	}

	return t.tmpl.Execute(file, data)
}
