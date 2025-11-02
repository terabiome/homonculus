package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/terabiome/homonculus/internal/cloudinit"
	"github.com/terabiome/homonculus/internal/config"
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/internal/disk"
	"github.com/terabiome/homonculus/internal/libvirt"
	"github.com/terabiome/homonculus/internal/provisioner"
	"github.com/terabiome/homonculus/pkg/constants"
	"github.com/terabiome/homonculus/pkg/logger"
	"github.com/terabiome/homonculus/pkg/templator"
	"github.com/urfave/cli/v2"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("homonculus starting", slog.String("log_level", cfg.LogLevel))

	app := &cli.App{
		Name:                 "homonculus",
		Usage:                "Provision and manage libvirt virtual machines",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:  "virtualmachine",
				Usage: "Execute VM-related functions",
				Action: func(c *cli.Context) error {
					fmt.Println("use subcommand instead:")
					for _, subcmd := range c.Command.Subcommands {
						fmt.Printf("\t - %s %s %s\n", c.App.Name, c.Command.Name, subcmd.Name)
					}
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "Create virtual machine(s)",
						Action: func(ctx *cli.Context) error {
							provisionerService, err := initProvisioner(cfg, log)
							if err != nil {
								return err
							}

							filepath := ctx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, err := os.Open(filepath)
							if err != nil {
								return err
							}
							defer f.Close()

							var clusterRequest contracts.CreateVirtualMachineClusterRequest
							if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
								return err
							}

							log.Info("creating VM cluster", slog.Int("count", len(clusterRequest.VirtualMachines)))

							if err := provisionerService.CreateCluster(clusterRequest); err != nil {
								return fmt.Errorf("unable to create virtual machines from template data: %w", err)
							}

							log.Info("VM cluster created successfully")
							return nil
						},
					},
					{
						Name:  "delete",
						Usage: "Delete virtual machine(s)",
						Action: func(ctx *cli.Context) error {
							provisionerService, err := initProvisioner(cfg, log)
							if err != nil {
								return err
							}

							filepath := ctx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, err := os.Open(filepath)
							if err != nil {
								return err
							}
							defer f.Close()

							var clusterRequest contracts.DeleteVirtualMachineClusterRequest
							if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
								return err
							}

							log.Info("deleting VM cluster", slog.Int("count", len(clusterRequest.VirtualMachines)))

							if err := provisionerService.DeleteCluster(clusterRequest); err != nil {
								return fmt.Errorf("unable to delete virtual machines from template data: %w", err)
							}

							log.Info("VM cluster deleted successfully")
							return nil
						},
					},
					{
						Name:  "clone",
						Usage: "Clone virtual machine(s)",
						Action: func(ctx *cli.Context) error {
							provisionerService, err := initProvisioner(cfg, log)
							if err != nil {
								return err
							}

							filepath := ctx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, err := os.Open(filepath)
							if err != nil {
								return err
							}
							defer f.Close()

							var clusterRequest contracts.CloneVirtualMachineClusterRequest
							if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
								return err
							}

							log.Info("cloning VM cluster",
								slog.String("base", clusterRequest.BaseVirtualMachine.Name),
								slog.Int("count", len(clusterRequest.TargetVirtualMachines)),
							)

							if err := provisionerService.CloneCluster(clusterRequest); err != nil {
								return fmt.Errorf("unable to clone virtual machines from template data: %w", err)
							}

							log.Info("VM cluster cloned successfully")
							return nil
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Error("application error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func initProvisioner(cfg *config.Config, log *slog.Logger) (*provisioner.Service, error) {
	engine := templator.NewEngine()

	log.Debug("loading templates")

	if err := engine.LoadTemplate(constants.TemplateLibvirt, cfg.LibvirtTemplatePath); err != nil {
		return nil, err
	}

	if err := engine.LoadTemplate(constants.TemplateCloudInitUserData, cfg.CloudInitUserDataTemplate); err != nil {
		return nil, err
	}

	if cfg.CloudInitMetaDataTemplate != "" {
		if err := engine.LoadTemplate(constants.TemplateCloudInitMetaData, cfg.CloudInitMetaDataTemplate); err != nil {
			return nil, err
		}
	}

	if cfg.CloudInitNetworkConfigTemplate != "" {
		if err := engine.LoadTemplate(constants.TemplateCloudInitNetworkConfig, cfg.CloudInitNetworkConfigTemplate); err != nil {
			return nil, err
		}
	}

	log.Debug("templates loaded successfully")

	return provisioner.NewService(
		disk.NewService(log),
		cloudinit.NewService(engine, log),
		libvirt.NewService(engine, log),
		log,
	), nil
}
