package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/terabiome/homonculus/pkg/executor"
)

// runSystemInfo displays NUMA topology and CPU information
func runSystemInfo() error {
	// Create local executor
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	exec := executor.NewLocal(logger)
	ctx := context.Background()

	fmt.Println("=== System Information ===")
	fmt.Println()

	// Show NUMA hardware info
	fmt.Println("NUMA Topology:")
	if err := runCommandWithExecutor(ctx, exec, "numactl", "--hardware"); err != nil {
		fmt.Println("  ⚠️  numactl not available, trying alternative...")
		// Fallback to lscpu
		if err := runCommandWithExecutor(ctx, exec, "lscpu"); err != nil {
			return fmt.Errorf("failed to get system info: %w", err)
		}
	}

	fmt.Println("\n---")
	fmt.Println("\nCPU Information:")
	if err := runCommandWithExecutor(ctx, exec, "lscpu"); err != nil {
		return fmt.Errorf("failed to get CPU info: %w", err)
	}

	return nil
}

// runCommandWithExecutor executes a command using the executor and prints its output
func runCommandWithExecutor(ctx context.Context, exec executor.Executor, command string, args ...string) error {
	var stdout, stderr bytes.Buffer

	exitCode, err := exec.Execute(ctx, &stdout, &stderr, command, args...)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("command failed: %w", err)
	}

	// Clean and print output
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fmt.Println("  " + line)
		}
	}

	return nil
}
