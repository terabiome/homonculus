package mkisofs

import (
	"context"
	"fmt"

	"github.com/terabiome/homonculus/pkg/executor"
)

type ISOOptions struct {
	OutputPath string
	VolumeID   string
	Files      []string
}

func CreateISO(ctx context.Context, exec executor.Executor, opts ISOOptions) error {
	args := []string{
		"-output", opts.OutputPath,
		"-volid", opts.VolumeID,
		"-joliet",
		"-r",
	}
	args = append(args, opts.Files...)

	result, err := executor.RunAndCapture(ctx, exec, "mkisofs", args...)
	if err != nil {
		return fmt.Errorf("mkisofs failed: %w\nstdout: %s\nstderr: %s",
			err, result.Stdout, result.Stderr)
	}

	return nil
}

