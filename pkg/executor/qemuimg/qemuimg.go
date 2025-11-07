package qemuimg

import (
	"context"
	"fmt"

	"github.com/terabiome/homonculus/pkg/executor"
)

type BackingImageOptions struct {
	BackingFile       string
	BackingFileFormat string
	OutputFile        string
	OutputFileFormat  string
	SizeGB            int64
}

func CreateBackingImage(ctx context.Context, exec executor.Executor, opts BackingImageOptions) error {
	args := []string{
		"create",
		"-b", opts.BackingFile,
		"-F", opts.BackingFileFormat,
		"-f", opts.OutputFileFormat,
		opts.OutputFile,
		fmt.Sprintf("%dG", opts.SizeGB),
	}

	result, err := executor.RunAndCapture(ctx, exec, "qemu-img", args...)
	if err != nil {
		return fmt.Errorf("qemu-img create failed: %w\nstdout: %s\nstderr: %s",
			err, result.Stdout, result.Stderr)
	}

	return nil
}

type InfoOptions struct {
	ImagePath string
}

func Info(ctx context.Context, exec executor.Executor, opts InfoOptions) (string, error) {
	result, err := executor.RunAndCapture(ctx, exec, "qemu-img", "info", opts.ImagePath)
	if err != nil {
		return "", fmt.Errorf("qemu-img info failed: %w\nstderr: %s", err, result.Stderr)
	}
	return result.Stdout, nil
}
