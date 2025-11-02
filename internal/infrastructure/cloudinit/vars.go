package cloudinit

import (
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/pkg/constants"
)

type UserDataTemplateVars struct {
	Hostname    string
	UserConfigs []api.UserConfig
	Role        constants.KubernetesRole
}

type MetaDataTemplateVars struct {
	InstanceID string
	Hostname   string
}

type NetworkConfigTemplateVars struct {
	Hostname string
}

