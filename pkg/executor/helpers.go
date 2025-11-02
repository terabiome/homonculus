package executor

import (
	"bytes"
	"context"
	"fmt"
)

func RunAndCapture(ctx context.Context, exec Executor, command string, args ...string) (*Result, error) {
	var outBuf, errBuf bytes.Buffer

	exitCode, err := exec.Execute(ctx, &outBuf, &errBuf, command, args...)

	return &Result{
		ExitCode: exitCode,
		Stdout:   outBuf.String(),
		Stderr:   errBuf.String(),
		Error:    err,
	}, err
}

func MustSucceed(ctx context.Context, exec Executor, command string, args ...string) {
	result, err := RunAndCapture(ctx, exec, command, args...)

	if err != nil || result.ExitCode != 0 {
		panic(fmt.Sprintf("command failed: %s %v\nexit code: %d\nstdout: %s\nstderr: %s\nerror: %v",
			command,
			args,
			result.ExitCode,
			result.Stdout,
			result.Stderr,
			err,
		))
	}
}

