package k3s

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/terabiome/homonculus/internal/api/contracts"
	"github.com/terabiome/homonculus/pkg/executor"
	"golang.org/x/sync/errgroup"
)

type LinePrefixer struct {
	prefix string
	dest   io.Writer
	mu     *sync.Mutex
}

func (p *LinePrefixer) Write(b []byte) (n int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	formatted := fmt.Sprintf("[%s] %s\n", p.prefix, string(b))
	_, err = fmt.Fprint(p.dest, formatted)
	return len(b), err
}

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
func (s *BootstrapService) BootstrapMasters(ctx context.Context, config contracts.K3sMasterBootstrapConfig) error {
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

// BootstrapWorkers installs K3s agent on one or more worker nodes in parallel.
func (s *BootstrapService) BootstrapWorkers(ctx context.Context, config contracts.K3sWorkerBootstrapConfig) error {
	s.logger.Info("starting K3s worker bootstrap (parallel)",
		slog.Int("nodes", len(config.Nodes)),
		slog.String("master_url", config.MasterURL),
	)

	// Use errgroup for parallel execution with proper error handling
	g, ctx := errgroup.WithContext(ctx)

	var writeMu sync.Mutex

	for i, node := range config.Nodes {
		// Capture loop variables
		i := i
		node := node

		g.Go(func() error {
			s.logger.Info("bootstrapping K3s worker",
				slog.Int("index", i+1),
				slog.Int("total", len(config.Nodes)),
				slog.String("host", node.Host),
			)

			nodeStdout := &LinePrefixer{prefix: node.Host, dest: s.stdout, mu: &writeMu}
			nodeStderr := &LinePrefixer{prefix: node.Host, dest: s.stderr, mu: &writeMu}

			if err := s.bootstrapWorker(ctx, node, config.Token, config.MasterURL, nodeStdout, nodeStderr); err != nil {
				s.logger.Error("failed to bootstrap worker",
					slog.String("host", node.Host),
					slog.String("error", err.Error()),
				)
				return fmt.Errorf("failed to bootstrap worker %s: %w", node.Host, err)
			}

			s.logger.Info("K3s worker bootstrapped successfully", slog.String("host", node.Host))
			return nil
		})
	}

	// Wait for all workers to complete
	if err := g.Wait(); err != nil {
		return err
	}

	s.logger.Info("K3s worker bootstrap complete", slog.Int("nodes", len(config.Nodes)))
	return nil
}

func (s *BootstrapService) bootstrapMaster(ctx context.Context, node contracts.K3sNodeConfig, token string) error {
	// Create SSH executor with persistent connection
	exec, err := s.createExecutor(node)
	if err != nil {
		return fmt.Errorf("failed to create SSH executor: %w", err)
	}
	defer exec.Close()

	// Use INSTALL_K3S_EXEC with environment variables as per K3s documentation
	// Reference: https://docs.k3s.io/installation/configuration#configuration-with-install-script
	cmd := fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC=\"server --cluster-init\" K3S_TOKEN=%s sh -s -", token)

	s.logger.Info("executing K3s master installation", slog.String("host", node.Host))

	// Stream output to configured writers (defaults to os.Stdout/os.Stderr)
	_, err = exec.Execute(ctx, s.stdout, s.stderr, cmd)

	if err != nil {
		s.logger.Error("master bootstrap failed", slog.String("host", node.Host))
		return err
	}

	return nil
}

func (s *BootstrapService) bootstrapWorker(ctx context.Context, node contracts.K3sNodeConfig, token, masterURL string, stdout, stderr io.Writer) error {
	// Create SSH executor with persistent connection
	exec, err := s.createExecutor(node)
	if err != nil {
		return fmt.Errorf("failed to create SSH executor: %w", err)
	}
	defer exec.Close()

	// Use INSTALL_K3S_EXEC with environment variables as per K3s documentation
	// Reference: https://docs.k3s.io/installation/configuration#configuration-with-install-script
	cmd := fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC=\"agent\" K3S_URL=%s K3S_TOKEN=%s sh -s -", masterURL, token)

	s.logger.Info("executing K3s worker installation", slog.String("host", node.Host))

	// Stream output to configured writers (defaults to os.Stdout/os.Stderr)
	_, err = exec.Execute(ctx, stdout, stderr, cmd)

	if err != nil {
		s.logger.Error("worker bootstrap failed", slog.String("host", node.Host))
		return err
	}

	return nil
}

func (s *BootstrapService) createExecutor(node contracts.K3sNodeConfig) (*executor.SSH, error) {
	return executor.NewSSH(executor.SSHConfig{
		Host:    node.Host,
		Port:    node.SSHPort,
		User:    node.SSHUser,
		KeyPath: node.SSHKey,
	}, s.logger)
}
