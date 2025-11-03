package k3s

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/pkg/executor"
)

// BootstrapService handles K3s cluster bootstrapping via SSH.
type BootstrapService struct {
	logger *slog.Logger
	stdout io.Writer
	stderr io.Writer
}

// NewBootstrapService creates a new K3s bootstrap service.
func NewBootstrapService(logger *slog.Logger) *BootstrapService {
	return &BootstrapService{
		logger: logger.With(slog.String("service", "k3s-bootstrap")),
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// WithOutput sets custom stdout/stderr writers for the bootstrap service.
// Useful for testing, logging to files, or capturing output.
func (s *BootstrapService) WithOutput(stdout, stderr io.Writer) *BootstrapService {
	s.stdout = stdout
	s.stderr = stderr
	return s
}

// BootstrapMasters installs K3s server on one or more master nodes.
func (s *BootstrapService) BootstrapMasters(ctx context.Context, config api.K3sMasterBootstrapConfig) error {
	s.logger.Info("starting K3s master bootstrap", slog.Int("nodes", len(config.Nodes)))

	for i, node := range config.Nodes {
		s.logger.Info("bootstrapping K3s master",
			slog.Int("index", i+1),
			slog.Int("total", len(config.Nodes)),
			slog.String("host", node.Host),
		)
		if err := s.bootstrapMaster(ctx, node, config.Token); err != nil {
			s.logger.Error("failed to bootstrap master",
				slog.String("host", node.Host),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("failed to bootstrap master %s: %w", node.Host, err)
		}
		s.logger.Info("K3s master bootstrapped successfully", slog.String("host", node.Host))
	}

	s.logger.Info("K3s master bootstrap complete", slog.Int("nodes", len(config.Nodes)))
	return nil
}

// BootstrapWorkers installs K3s agent on one or more worker nodes.
func (s *BootstrapService) BootstrapWorkers(ctx context.Context, config api.K3sWorkerBootstrapConfig) error {
	s.logger.Info("starting K3s worker bootstrap",
		slog.Int("nodes", len(config.Nodes)),
		slog.String("master_url", config.MasterURL),
	)

	for i, node := range config.Nodes {
		s.logger.Info("bootstrapping K3s worker",
			slog.Int("index", i+1),
			slog.Int("total", len(config.Nodes)),
			slog.String("host", node.Host),
		)
		if err := s.bootstrapWorker(ctx, node, config.Token, config.MasterURL); err != nil {
			s.logger.Error("failed to bootstrap worker",
				slog.String("host", node.Host),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("failed to bootstrap worker %s: %w", node.Host, err)
		}
		s.logger.Info("K3s worker bootstrapped successfully", slog.String("host", node.Host))
	}

	s.logger.Info("K3s worker bootstrap complete", slog.Int("nodes", len(config.Nodes)))
	return nil
}

func (s *BootstrapService) bootstrapMaster(ctx context.Context, node api.K3sNodeConfig, token string) error {
	// Create SSH executor with persistent connection
	exec, err := s.createExecutor(node)
	if err != nil {
		return fmt.Errorf("failed to create SSH executor: %w", err)
	}
	defer exec.Close()

	// Pass entire command as single string to avoid shell quoting issues
	cmd := fmt.Sprintf("curl -sfL https://get.k3s.io | K3S_TOKEN=%s sh -s - server --cluster-init", token)

	s.logger.Info("executing K3s master installation", slog.String("host", node.Host))

	// Stream output to configured writers (defaults to os.Stdout/os.Stderr)
	_, err = exec.Execute(ctx, s.stdout, s.stderr, cmd)

	if err != nil {
		s.logger.Error("master bootstrap failed", slog.String("host", node.Host))
		return err
	}

	return nil
}

func (s *BootstrapService) bootstrapWorker(ctx context.Context, node api.K3sNodeConfig, token, masterURL string) error {
	// Create SSH executor with persistent connection
	exec, err := s.createExecutor(node)
	if err != nil {
		return fmt.Errorf("failed to create SSH executor: %w", err)
	}
	defer exec.Close()

	// Pass entire command as single string to avoid shell quoting issues
	cmd := fmt.Sprintf("curl -sfL https://get.k3s.io | K3S_TOKEN=%s K3S_URL=%s sh -s - agent", token, masterURL)

	s.logger.Info("executing K3s worker installation", slog.String("host", node.Host))

	// Stream output to configured writers (defaults to os.Stdout/os.Stderr)
	_, err = exec.Execute(ctx, s.stdout, s.stderr, cmd)

	if err != nil {
		s.logger.Error("worker bootstrap failed", slog.String("host", node.Host))
		return err
	}

	return nil
}

func (s *BootstrapService) createExecutor(node api.K3sNodeConfig) (*executor.SSH, error) {
	return executor.NewSSH(executor.SSHConfig{
		Host:    node.Host,
		Port:    node.SSHPort,
		User:    node.SSHUser,
		KeyPath: node.SSHKey,
	}, s.logger)
}
