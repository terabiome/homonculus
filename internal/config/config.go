package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	LibvirtTemplatePath            string
	CloudInitUserDataTemplate      string
	CloudInitMetaDataTemplate      string
	CloudInitNetworkConfigTemplate string
	LogLevel                       string
}

func Load() (*Config, error) {
	viper.SetDefault("libvirt_template", "./templates/libvirt/domain.xml.tpl")
	viper.SetDefault("cloudinit_user_data_template", "./templates/cloudinit/user-data.tpl")
	viper.SetDefault("cloudinit_meta_data_template", "./templates/cloudinit/meta-data.tpl")
	viper.SetDefault("cloudinit_network_config_template", "./templates/cloudinit/network-config.tpl")
	viper.SetDefault("log_level", "info")

	viper.SetEnvPrefix("homonculus")
	viper.AutomaticEnv()

	cfg := &Config{
		LibvirtTemplatePath:            viper.GetString("libvirt_template"),
		CloudInitUserDataTemplate:      viper.GetString("cloudinit_user_data_template"),
		CloudInitMetaDataTemplate:      viper.GetString("cloudinit_meta_data_template"),
		CloudInitNetworkConfigTemplate: viper.GetString("cloudinit_network_config_template"),
		LogLevel:                       viper.GetString("log_level"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if err := validateFileExists(c.LibvirtTemplatePath); err != nil {
		return fmt.Errorf("libvirt template: %w", err)
	}

	if err := validateFileExists(c.CloudInitUserDataTemplate); err != nil {
		return fmt.Errorf("cloud-init user-data template: %w", err)
	}

	if c.CloudInitMetaDataTemplate != "" {
		if err := validateFileExists(c.CloudInitMetaDataTemplate); err != nil {
			return fmt.Errorf("cloud-init meta-data template: %w", err)
		}
	}

	if c.CloudInitNetworkConfigTemplate != "" {
		if err := validateFileExists(c.CloudInitNetworkConfigTemplate); err != nil {
			return fmt.Errorf("cloud-init network-config template: %w", err)
		}
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (valid: debug, info, warn, error)", c.LogLevel)
	}

	return nil
}

func validateFileExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", path)
	} else if err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}
	return nil
}
