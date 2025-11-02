package fileops

import (
	"context"
	"fmt"

	"github.com/terabiome/homonculus/pkg/executor"
)

func RemoveFile(ctx context.Context, exec executor.Executor, path string) error {
	result, err := executor.RunAndCapture(ctx, exec, "rm", "-f", path)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w\nstderr: %s", path, err, result.Stderr)
	}
	return nil
}

func RemoveDirectory(ctx context.Context, exec executor.Executor, path string) error {
	result, err := executor.RunAndCapture(ctx, exec, "rm", "-rf", path)
	if err != nil {
		return fmt.Errorf("failed to remove directory %s: %w\nstderr: %s", path, err, result.Stderr)
	}
	return nil
}

func CreateDirectory(ctx context.Context, exec executor.Executor, path string) error {
	result, err := executor.RunAndCapture(ctx, exec, "mkdir", "-p", path)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w\nstderr: %s", path, err, result.Stderr)
	}
	return nil
}

func CopyFile(ctx context.Context, exec executor.Executor, src, dst string) error {
	result, err := executor.RunAndCapture(ctx, exec, "cp", src, dst)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w\nstderr: %s", src, dst, err, result.Stderr)
	}
	return nil
}

func MoveFile(ctx context.Context, exec executor.Executor, src, dst string) error {
	result, err := executor.RunAndCapture(ctx, exec, "mv", src, dst)
	if err != nil {
		return fmt.Errorf("failed to move %s to %s: %w\nstderr: %s", src, dst, err, result.Stderr)
	}
	return nil
}

