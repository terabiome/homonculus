package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/terabiome/homonculus/internal/adapter"
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/config"
	"github.com/terabiome/homonculus/internal/handler"
	"github.com/terabiome/homonculus/internal/infrastructure/cloudinit"
	"github.com/terabiome/homonculus/internal/infrastructure/disk"
	"github.com/terabiome/homonculus/internal/infrastructure/libvirt"
	"github.com/terabiome/homonculus/internal/routes"
	"github.com/terabiome/homonculus/internal/service"
	"github.com/terabiome/homonculus/pkg/constants"
	"github.com/terabiome/homonculus/pkg/k3s"
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
				Name:  "server",
				Usage: "Start HTTP API server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "address",
						Aliases: []string{"a"},
						Usage:   "Server address",
						Value:   ":8080",
					},
				},
				Action: func(cliCtx *cli.Context) error {
					return runServer(ctx, cfg, log, cliCtx.String("address"))
				},
			},
			{
				Name:  "system",
				Usage: "Show system information",
				Subcommands: []*cli.Command{
					{
						Name:  "cpu-topology",
						Usage: "Display CPU and NUMA topology information",
						Action: func(cliCtx *cli.Context) error {
							return runSystemInfo()
						},
					},
				},
			},
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
							vmParams := adapter.AdaptCreateCluster(clusterRequest)

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
							vmParams := adapter.AdaptDeleteCluster(clusterRequest)

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
							vmParams := adapter.AdaptStartCluster(clusterRequest)

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
								vmParams = adapter.AdaptQueryCluster(clusterRequest)
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
								VirtualMachines: adapter.AdaptVMInfoToAPI(vmInfos),
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
							cloneParams := adapter.AdaptCloneCluster(clusterRequest)

							if err := vmService.CloneCluster(ctx, cloneParams); err != nil {
								return fmt.Errorf("unable to clone virtual machines from template data: %w", err)
							}

							log.Info("VM cluster cloned successfully")
							return nil
						},
					},
				},
			},
			{
				Name:  "k3s",
				Usage: "K3s cluster management",
				Subcommands: []*cli.Command{
					{
						Name:  "generate-token",
						Usage: "Generate a secure K3s cluster token",
						Action: func(cliCtx *cli.Context) error {
							token, err := k3s.GenerateToken()
							if err != nil {
								return fmt.Errorf("failed to generate token: %w", err)
							}
							fmt.Println(token)
							return nil
						},
					},
					{
						Name:  "bootstrap",
						Usage: "Bootstrap K3s cluster nodes",
						Subcommands: []*cli.Command{
							{
								Name:  "master",
								Usage: "Bootstrap K3s master/server node(s)",
								Action: func(cliCtx *cli.Context) error {
									filepath := cliCtx.Args().First()
									if filepath == "" {
										return errors.New("empty file path to K3s master bootstrap config")
									}

									f, err := os.Open(filepath)
									if err != nil {
										return err
									}
									defer f.Close()

									var config api.K3sMasterBootstrapConfig
									if err := json.NewDecoder(f).Decode(&config); err != nil {
										return err
									}

									// Validate token
									if config.Token == "" {
										return errors.New("token is required in bootstrap config")
									}

									log.Info("starting K3s master bootstrap", slog.Int("nodes", len(config.Nodes)))

									bootstrapService := k3s.NewBootstrapService(log)
									if err := bootstrapService.BootstrapMasters(ctx, config); err != nil {
										return fmt.Errorf("master bootstrap failed: %w", err)
									}

									log.Info("K3s master bootstrap completed successfully")
									return nil
								},
							},
							{
								Name:  "worker",
								Usage: "Bootstrap K3s worker/agent node(s)",
								Action: func(cliCtx *cli.Context) error {
									filepath := cliCtx.Args().First()
									if filepath == "" {
										return errors.New("empty file path to K3s worker bootstrap config")
									}

									f, err := os.Open(filepath)
									if err != nil {
										return err
									}
									defer f.Close()

									var config api.K3sWorkerBootstrapConfig
									if err := json.NewDecoder(f).Decode(&config); err != nil {
										return err
									}

									// Validate token and master URL
									if config.Token == "" {
										return errors.New("token is required in bootstrap config")
									}
									if config.MasterURL == "" {
										return errors.New("master_url is required in bootstrap config")
									}

									log.Info("starting K3s worker bootstrap",
										slog.Int("nodes", len(config.Nodes)),
										slog.String("master_url", config.MasterURL),
									)

									bootstrapService := k3s.NewBootstrapService(log)
									if err := bootstrapService.BootstrapWorkers(ctx, config); err != nil {
										return fmt.Errorf("worker bootstrap failed: %w", err)
									}

									log.Info("K3s worker bootstrap completed successfully")
									return nil
								},
							},
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

	connManager, err := pkglibvirt.NewConnectionManager(cfg.LibvirtURI, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize connection manager: %w", err)
	}
	log.Info("connection manager initialized", slog.String("uri", cfg.LibvirtURI))

	return service.NewVMService(
		disk.NewManager(log),
		cloudinit.NewManager(engine, log),
		libvirt.NewManager(engine, log),
		connManager,
		log,
	), nil
}

// runServer starts the HTTP API server
func runServer(ctx context.Context, cfg *config.Config, log *slog.Logger, address string) error {
	log.Info("initializing HTTP server", slog.String("address", address))

	// Initialize VM service
	vmService, err := initVMService(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to initialize VM service: %w", err)
	}

	// Initialize handlers
	vmHandler := handler.NewVirtualMachine(vmService, log)
	k3sHandler := handler.NewK3s(log)
	systemHandler := handler.NewSystem(log)

	// Setup router
	router := routes.SetupMux(vmHandler, k3sHandler, systemHandler)

	// Create HTTP server
	server := &http.Server{
		Addr:         address,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		log.Info("HTTP server starting", slog.String("address", address))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrChan:
		return err
	case <-ctx.Done():
		log.Info("shutting down HTTP server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}
		log.Info("HTTP server stopped")
		return nil
	}
}
