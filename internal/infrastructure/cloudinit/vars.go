package cloudinit

import (
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/pkg/constants"
)

type UserDataTemplateVars struct {
	Hostname    string
	UserConfigs []api.UserConfig
	Role        constants.KubernetesRole
	Runcmds     []string
}

type MetaDataTemplateVars struct {
	InstanceID string
	Hostname   string
}

type NetworkConfigTemplateVars struct {
	Hostname           string
	IPv4Address        string
	IPv4GatewayAddress string
}
