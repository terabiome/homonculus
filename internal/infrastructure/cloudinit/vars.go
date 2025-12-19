package cloudinit

import (
	"github.com/terabiome/homonculus/internal/api/contracts"
	"github.com/terabiome/homonculus/pkg/constants"
)

type UserDataTemplateVars struct {
	Hostname         string
	UserConfigs      []contracts.UserConfig
	Role             constants.KubernetesRole
	DoPackageUpdate  bool
	DoPackageUpgrade bool
	Runcmds          []string
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
