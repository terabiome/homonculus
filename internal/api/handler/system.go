package handler

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"

	"github.com/terabiome/homonculus/pkg/executor"
)

// System handles system-related HTTP requests
type System struct {
	logger *slog.Logger
}

// NewSystem creates a new System handler
func NewSystem(logger *slog.Logger) *System {
	return &System{
		logger: logger,
	}
}

// CPUTopology handles GET /cpu-topology requests to display CPU and NUMA topology
func (h *System) CPUTopology(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	// Create local executor
	exec := executor.NewLocal(h.logger)

	var result struct {
		NUMATopology string `json:"numa_topology,omitempty"`
		CPUInfo      string `json:"cpu_info,omitempty"`
		Error        string `json:"error,omitempty"`
	}

	// Get NUMA topology
	var numaOutput bytes.Buffer
	var numaStderr bytes.Buffer
	numaExitCode, numaErr := exec.Execute(ctx, &numaOutput, &numaStderr, "numactl", "--hardware")
	if numaErr != nil || numaExitCode != 0 {
		// Fallback to lscpu
		h.logger.Debug("numactl not available, trying lscpu")
		var lscpuOutput bytes.Buffer
		var lscpuStderr bytes.Buffer
		lscpuExitCode, lscpuErr := exec.Execute(ctx, &lscpuOutput, &lscpuStderr, "lscpu")
		if lscpuErr != nil || lscpuExitCode != 0 {
			writeResult(writer, http.StatusInternalServerError, GenericResponse{
				Body:    nil,
				Message: "failed to get system information",
				Error:   "both numactl and lscpu failed",
			})
			return
		}
		result.NUMATopology = cleanOutput(lscpuOutput.String())
	} else {
		result.NUMATopology = cleanOutput(numaOutput.String())
	}

	// Get CPU info
	var cpuOutput bytes.Buffer
	var cpuStderr bytes.Buffer
	cpuExitCode, cpuErr := exec.Execute(ctx, &cpuOutput, &cpuStderr, "lscpu")
	if cpuErr != nil || cpuExitCode != 0 {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to get CPU information",
			Error:   cpuStderr.String(),
		})
		return
	}
	result.CPUInfo = cleanOutput(cpuOutput.String())

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    result,
		Message: "retrieved CPU and NUMA topology successfully",
	})
}

// cleanOutput removes empty lines from command output
func cleanOutput(output string) string {
	lines := strings.Split(output, "\n")
	var cleaned []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

