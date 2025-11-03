package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/config"
	"github.com/terabiome/homonculus/internal/infrastructure/cloudinit"
	"github.com/terabiome/homonculus/internal/infrastructure/disk"
	"github.com/terabiome/homonculus/internal/infrastructure/libvirt"
	"github.com/terabiome/homonculus/internal/service"
	"github.com/terabiome/homonculus/pkg/constants"
	pkglibvirt "github.com/terabiome/homonculus/pkg/libvirt"
	"github.com/terabiome/homonculus/pkg/logger"
	"github.com/terabiome/homonculus/pkg/telemetry"
	"github.com/terabiome/homonculus/pkg/templator"
	"github.com/urfave/cli/v2"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.LogFormat)
	log.Info("homonculus starting",
		slog.String("log_level", cfg.LogLevel),
		slog.String("log_format", cfg.LogFormat),
		slog.Bool("telemetry_enabled", cfg.TelemetryEnabled),
	)

	var tel *telemetry.Telemetry
	if cfg.TelemetryEnabled {
		var err error
		tel, err = telemetry.Initialize("homonculus")
		if err != nil {
			log.Error("failed to initialize telemetry", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer func() {
			log.Info("shutting down telemetry")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := tel.Shutdown(shutdownCtx); err != nil {
				log.Error("failed to shutdown telemetry", slog.String("error", err.Error()))
			}
		}()
		log.Info("telemetry initialized")
	} else {
		log.Debug("telemetry disabled")
	}

	go func() {
		sig := <-sigChan
		log.Info("received shutdown signal", slog.String("signal", sig.String()))
		cancel()
	}()

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
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "start",
								Aliases: []string{"s"},
								Usage:   "Start VMs after creation",
								Value:   false,
							},
						},
						Action: func(cliCtx *cli.Context) error {
							vmService, err := initVMService(cfg, log)
							if err != nil {
								return err
							}

							filepath := cliCtx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, err := os.Open(filepath)
							if err != nil {
								return err
							}
							defer f.Close()

							var clusterRequest api.CreateClusterRequest
							if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
								return err
							}

							log.Info("creating VM cluster", slog.Int("count", len(clusterRequest.VirtualMachines)))

							// Adapt CLI contract to service params
							vmParams := adaptCreateCluster(clusterRequest)

							// Prepare start params if flag is set
							var startVMs []service.StartVMParams
							if cliCtx.Bool("start") {
								startVMs = make([]service.StartVMParams, len(clusterRequest.VirtualMachines))
								for i, vm := range clusterRequest.VirtualMachines {
									startVMs[i] = service.StartVMParams{Name: vm.Name}
								}
							}

							if err := vmService.CreateCluster(ctx, vmParams); err != nil {
								return fmt.Errorf("unable to create virtual machines from template data: %w", err)
							}

							log.Info("VM cluster created successfully")

							// Start VMs if we prepared the list
							if len(startVMs) > 0 {
								log.Info("starting VM cluster", slog.Int("count", len(startVMs)))

								if err := vmService.StartCluster(ctx, startVMs); err != nil {
									return fmt.Errorf("unable to start virtual machines: %w", err)
								}

								log.Info("VM cluster started successfully")
							}

							return nil
						},
					},
					{
						Name:  "delete",
						Usage: "Delete virtual machine(s)",
						Action: func(cliCtx *cli.Context) error {
							vmService, err := initVMService(cfg, log)
							if err != nil {
								return err
							}

							filepath := cliCtx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, err := os.Open(filepath)
							if err != nil {
								return err
							}
							defer f.Close()

							var clusterRequest api.DeleteClusterRequest
							if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
								return err
							}

							log.Info("deleting VM cluster", slog.Int("count", len(clusterRequest.VirtualMachines)))

							// Adapt CLI contract to service params
							vmParams := adaptDeleteCluster(clusterRequest)

							if err := vmService.DeleteCluster(ctx, vmParams); err != nil {
								return fmt.Errorf("unable to delete virtual machines from template data: %w", err)
							}

							log.Info("VM cluster deleted successfully")
							return nil
						},
					},
					{
						Name:  "start",
						Usage: "Start virtual machine(s)",
						Action: func(cliCtx *cli.Context) error {
							vmService, err := initVMService(cfg, log)
							if err != nil {
								return err
							}

							filepath := cliCtx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, err := os.Open(filepath)
							if err != nil {
								return err
							}
							defer f.Close()

							var clusterRequest api.StartClusterRequest
							if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
								return err
							}

							log.Info("starting VM cluster", slog.Int("count", len(clusterRequest.VirtualMachines)))

							// Adapt CLI contract to service params
							vmParams := adaptStartCluster(clusterRequest)

							if err := vmService.StartCluster(ctx, vmParams); err != nil {
								return fmt.Errorf("unable to start virtual machines from template data: %w", err)
							}

							log.Info("VM cluster started successfully")
							return nil
						},
					},
					{
						Name:  "query",
						Usage: "Query virtual machine(s) information. Omit file path to list all VMs.",
						Action: func(cliCtx *cli.Context) error {
							vmService, err := initVMService(cfg, log)
							if err != nil {
								return err
							}

							filepath := cliCtx.Args().First()

							var vmParams []service.QueryVMParams

							// If filepath provided, query specific VMs
							if filepath != "" {
								f, err := os.Open(filepath)
								if err != nil {
									return err
								}
								defer f.Close()

								var clusterRequest api.QueryClusterRequest
								if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
									return err
								}

								log.Info("querying VM cluster", slog.Int("count", len(clusterRequest.VirtualMachines)))
								vmParams = adaptQueryCluster(clusterRequest)
							} else {
								// No filepath provided, list all VMs
								log.Info("listing all VMs")
								vmParams = nil // Empty slice will trigger list all
							}

							vmInfos, err := vmService.QueryCluster(ctx, vmParams)
							if err != nil {
								log.Warn("some VMs failed to query", slog.String("error", err.Error()))
							}

							// Convert service VMInfo to API VMInfo
							response := api.QueryClusterResponse{
								VirtualMachines: adaptVMInfoToAPI(vmInfos),
							}

							// Output as JSON
							output, err := json.MarshalIndent(response, "", "  ")
							if err != nil {
								return fmt.Errorf("unable to marshal response: %w", err)
							}

							fmt.Println(string(output))
							return nil
						},
					},
					{
						Name:  "clone",
						Usage: "Clone virtual machine(s)",
						Action: func(cliCtx *cli.Context) error {
							vmService, err := initVMService(cfg, log)
							if err != nil {
								return err
							}

							filepath := cliCtx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, err := os.Open(filepath)
							if err != nil {
								return err
							}
							defer f.Close()

							var clusterRequest api.CloneClusterRequest
							if err := json.NewDecoder(f).Decode(&clusterRequest); err != nil {
								return err
							}

							log.Info("cloning VM cluster",
								slog.String("base", clusterRequest.BaseVM.Name),
								slog.Int("count", len(clusterRequest.TargetVMs)),
							)

							// Adapt CLI contract to service params
							cloneParams := adaptCloneCluster(clusterRequest)

							if err := vmService.CloneCluster(ctx, cloneParams); err != nil {
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

func initVMService(cfg *config.Config, log *slog.Logger) (*service.VMService, error) {
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

	connManager, err := pkglibvirt.NewConnectionManager(log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize connection manager: %w", err)
	}
	log.Info("connection manager initialized")

	return service.NewVMService(
		disk.NewManager(log),
		cloudinit.NewManager(engine, log),
		libvirt.NewManager(engine, log),
		connManager,
		log,
	), nil
}
