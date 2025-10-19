package templator

import (
	"fmt"
	"text/template"

	"github.com/google/uuid"
)

type LibvirtTemplatePlaceholder struct {
	Name             string
	UUID             uuid.UUID
	MemoryKiB        int64
	VCPU             int
	DiskPath         string
	CloudInitISOPath string
}

type LibvirtTemplator struct {
	*Templator
}

func NewLibvirtTemplator(templatePath string) (*LibvirtTemplator, error) {
	t := &LibvirtTemplator{&Templator{}}

	tmpl, err := template.New("libvirt-domain.xml.tpl").ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("could not parse libvirt template file %s: %w", templatePath, err)
	}

	t.SetTemplate(tmpl)
	return t, nil
}

func (t *LibvirtTemplator) ToFile(filepath string, data LibvirtTemplatePlaceholder) error {
	return t.Templator.ToFile(filepath, data)
}
