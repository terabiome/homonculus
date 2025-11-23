package templator

import (
	"fmt"
	"text/template"

	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/pkg/constants"
)

type CloudInitTemplatePlaceholder struct {
	Hostname    string
	UserConfigs []api.UserConfig
	Role        constants.KubernetesRole
	Runcmds     []string
}

type CloudInitTemplator struct {
	*Templator
}

func NewCloudInitTemplator(templatePath string) (*CloudInitTemplator, error) {
	t := &CloudInitTemplator{&Templator{}}

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("could not parse cloud-init template file %s: %w", templatePath, err)
	}

	t.SetTemplate(tmpl)
	return t, nil
}

func (t *CloudInitTemplator) ToFile(filepath string, data CloudInitTemplatePlaceholder) error {
	return t.Templator.ToFile(filepath, data)
}
