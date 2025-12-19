package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/terabiome/homonculus/internal/api/handler"
	"github.com/terabiome/homonculus/internal/api/routes"
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
