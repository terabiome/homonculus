package executor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSH executes commands on a remote host via SSH.
// It maintains a persistent connection that can be reused across multiple Execute calls.
type SSH struct {
	client *ssh.Client
	host   string
	logger *slog.Logger
}

// SSHConfig contains SSH connection parameters.
type SSHConfig struct {
	Host    string
	Port    int
	User    string
	KeyPath string
}

// NewSSH creates a new SSH executor with an established connection.
func NewSSH(config SSHConfig, logger *slog.Logger) (*SSH, error) {
	log := logger.With(slog.String("executor", "ssh"), slog.String("host", config.Host))

	client, err := createSSHClient(config, log)
	if err != nil {
		return nil, err
	}

	return &SSH{
		client: client,
		host:   config.Host,
		logger: log,
	}, nil
}

// Close closes the SSH connection.
func (e *SSH) Close() error {
	if e.client != nil {
		e.logger.Debug("closing SSH connection")
		return e.client.Close()
	}
	return nil
}

func (e *SSH) Name() string {
	return fmt.Sprintf("ssh-%s", e.host)
}

func (e *SSH) Execute(
	ctx context.Context,
	stdout, stderr io.Writer,
	command string, args ...string,
) (int, error) {
	cmdStr := e.buildCommandString(command, args)
	e.logger.Debug("executing command via SSH", slog.String("cmd", cmdStr))

	// Create session
	session, err := e.client.NewSession()
	if err != nil {
		return -1, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Set up output streams
	session.Stdout = stdout
	session.Stderr = stderr

	// Execute command
	err = session.Run(cmdStr)
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode := exitErr.ExitStatus()
			e.logger.Warn("SSH command failed",
				slog.String("cmd", cmdStr),
				slog.Int("exit_code", exitCode),
			)
			return exitCode, fmt.Errorf("command exited with code %d: %w", exitCode, err)
		}

		e.logger.Error("SSH command execution error",
			slog.String("cmd", cmdStr),
			slog.String("error", err.Error()),
		)
		return -1, fmt.Errorf("command execution failed: %w", err)
	}

	e.logger.Debug("SSH command succeeded", slog.String("cmd", cmdStr))
	return 0, nil
}

func (e *SSH) buildCommandString(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return command + " " + strings.Join(args, " ")
}

// createSSHClient establishes an SSH connection from the given config.
func createSSHClient(config SSHConfig, logger *slog.Logger) (*ssh.Client, error) {
	port := config.Port
	if port == 0 {
		port = 22
	}

	// Expand ~ in SSH key path
	keyPath := config.KeyPath
	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(home, keyPath[2:])
	}

	// Read SSH private key
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key %s: %w", keyPath, err)
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}

	// Configure SSH client
	sshConfig := &ssh.ClientConfig{
		User: config.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Make this configurable
	}

	// Establish connection
	addr := fmt.Sprintf("%s:%d", config.Host, port)
	logger.Debug("establishing SSH connection", slog.String("addr", addr))

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	logger.Debug("SSH connection established", slog.String("addr", addr))
	return client, nil
}
