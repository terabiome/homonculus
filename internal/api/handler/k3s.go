package handler

import (
	"log/slog"
	"net/http"

	"github.com/terabiome/homonculus/internal/api/contracts"
	"github.com/terabiome/homonculus/pkg/k3s"
)

// K3s handles K3s-related HTTP requests
type K3s struct {
	logger *slog.Logger
}

// NewK3s creates a new K3s handler
func NewK3s(logger *slog.Logger) *K3s {
	return &K3s{
		logger: logger,
	}
}

// GenerateToken handles POST /generate-token requests to generate a K3s token
func (h *K3s) GenerateToken(writer http.ResponseWriter, request *http.Request) {
	token, err := k3s.GenerateToken()
	if err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to generate K3s token",
			Error:   err.Error(),
		})
		return
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body: map[string]string{
			"token": token,
		},
		Message: "generated K3s token successfully",
	})
}

// BootstrapMaster handles POST /bootstrap/master requests to bootstrap K3s master nodes
func (h *K3s) BootstrapMaster(writer http.ResponseWriter, request *http.Request) {
	var config contracts.K3sMasterBootstrapConfig
	cb, err := parseBodyAndHandleError(writer, request, &config, true)
	if err != nil {
		cb()
		return
	}

	// Validate token
	if config.Token == "" {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "token is required in bootstrap config",
		})
		return
	}

	if len(config.Nodes) == 0 {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "no nodes specified in bootstrap config",
		})
		return
	}

	ctx := request.Context()
	bootstrapService := k3s.NewBootstrapService(h.logger)
	if err := bootstrapService.BootstrapMasters(ctx, config); err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to bootstrap K3s master nodes",
			Error:   err.Error(),
		})
		return
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    config,
		Message: "bootstrapped K3s master nodes successfully",
	})
}

// BootstrapWorker handles POST /bootstrap/worker requests to bootstrap K3s worker nodes
func (h *K3s) BootstrapWorker(writer http.ResponseWriter, request *http.Request) {
	var config contracts.K3sWorkerBootstrapConfig
	cb, err := parseBodyAndHandleError(writer, request, &config, true)
	if err != nil {
		cb()
		return
	}

	// Validate token and master URL
	if config.Token == "" {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "token is required in bootstrap config",
		})
		return
	}

	if config.MasterURL == "" {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "master_url is required in bootstrap config",
		})
		return
	}

	if len(config.Nodes) == 0 {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "no nodes specified in bootstrap config",
		})
		return
	}

	ctx := request.Context()
	bootstrapService := k3s.NewBootstrapService(h.logger)
	if err := bootstrapService.BootstrapWorkers(ctx, config); err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to bootstrap K3s worker nodes",
			Error:   err.Error(),
		})
		return
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    config,
		Message: "bootstrapped K3s worker nodes successfully",
	})
}
