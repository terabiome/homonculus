package templator

import (
	"fmt"
	"text/template"

	"github.com/google/uuid"
)

type LibvirtTemplatePlaceholder struct {
	Name                   string
	UUID                   uuid.UUID
	MemoryKiB              int64
	VCPU                   int
	BridgeNetworkInterface string
	DiskPath               string
	CloudInitISOPath       string
}

type LibvirtTemplator struct {
	*Templator
}

func NewLibvirtTemplator(templatePath string) (*LibvirtTemplator, error) {
	t := &LibvirtTemplator{&Templator{}}

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("could not parse libvirt template file %s: %w", templatePath, err)
	}

	t.SetTemplate(tmpl)
	return t, nil
}

func (t *LibvirtTemplator) ToBytes(data LibvirtTemplatePlaceholder) ([]byte, error) {
	return t.Templator.ToBytes(data)
}
