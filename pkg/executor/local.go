package executor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

type Local struct {
	logger *slog.Logger
}

func NewLocal(logger *slog.Logger) *Local {
	return &Local{
		logger: logger,
	}
}

func (e *Local) Name() string {
	return "local-shell"
}

func (e *Local) Execute(
	ctx context.Context,
	stdout, stderr io.Writer,
	command string, args ...string,
) (int, error) {
	cmdStr := e.buildCommandString(command, args)
	e.logger.Debug("executing command locally", slog.String("cmd", cmdStr))

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			e.logger.Warn("command failed",
				slog.String("cmd", cmdStr),
				slog.Int("exit_code", exitCode),
			)
			return exitCode, fmt.Errorf("command exited with code %d: %w", exitCode, err)
		}

		e.logger.Error("command execution error",
			slog.String("cmd", cmdStr),
			slog.String("error", err.Error()),
		)
		return -1, fmt.Errorf("command execution failed: %w", err)
	}

	e.logger.Debug("command succeeded", slog.String("cmd", cmdStr))
	return 0, nil
}

func (e *Local) buildCommandString(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return command + " " + strings.Join(args, " ")
}

