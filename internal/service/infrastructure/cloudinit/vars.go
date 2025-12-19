package cloudinit

import "github.com/terabiome/homonculus/internal/service/parameters"

type UserDataTemplateVars struct {
	Hostname         string
	UserConfigs      []parameters.UserConfig
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
