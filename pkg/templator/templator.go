package templator

import (
	"bytes"
	"fmt"
	"os"
)

type Templator struct {
	tmpl Template
}

func (t *Templator) SetTemplate(tmpl Template) {
	t.tmpl = tmpl
}

func (t *Templator) ToFile(filepath string, data any) error {
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("could not open file %s: %w", filepath, err)
	}
	defer file.Close()

	return t.tmpl.Execute(file, data)
}

func (t *Templator) ToBytes(data any) ([]byte, error) {
	buffer := bytes.NewBuffer([]byte{})
	if err := t.tmpl.Execute(buffer, data); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
