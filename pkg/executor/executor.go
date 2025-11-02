package executor

import (
	"context"
	"io"
)

type Executor interface {
	Execute(ctx context.Context, stdout, stderr io.Writer, command string, args ...string) (exitCode int, err error)
	Name() string
}

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Error    error
}
