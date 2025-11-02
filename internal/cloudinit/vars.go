package cloudinit

import (
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/pkg/constants"
)

type UserDataTemplateVars struct {
	Hostname    string
	UserConfigs []contracts.UserConfig
	Role        constants.KubernetesRole
}

type MetaDataTemplateVars struct {
	InstanceID string
	Hostname   string
}

type NetworkConfigTemplateVars struct {
	Hostname string
}
