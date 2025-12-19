package handler

import (
	"log/slog"
	"net/http"

	"github.com/terabiome/homonculus/internal/adapter"
	"github.com/terabiome/homonculus/internal/api/contracts"
	"github.com/terabiome/homonculus/internal/service"
	"github.com/terabiome/homonculus/internal/service/parameters"
)

// VirtualMachine handles VM-related HTTP requests
type VirtualMachine struct {
	vmService *service.VMService
	logger    *slog.Logger
	spAdapter *adapter.ServiceParameterAdapter
}

// NewVirtualMachine creates a new VirtualMachine handler
func NewVirtualMachine(vmService *service.VMService, logger *slog.Logger, spAdapter *adapter.ServiceParameterAdapter) *VirtualMachine {
	return &VirtualMachine{
		vmService: vmService,
		logger:    logger,
		spAdapter: spAdapter,
	}
}

// CreateCluster handles POST /create/cluster requests to create multiple VMs
func (h *VirtualMachine) CreateCluster(writer http.ResponseWriter, request *http.Request) {
	var createRequest contracts.CreateClusterRequest
	cb, err := parseBodyAndHandleError(writer, request, &createRequest, true)
	if err != nil {
		cb()
		return
	}

	if len(createRequest.VirtualMachines) == 0 {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "no virtual machines specified in request",
		})
		return
	}

	// Adapt API contract to service params
	vmParams := h.spAdapter.AdaptCreateCluster(createRequest)

	ctx := request.Context()
	if err := h.vmService.CreateCluster(ctx, vmParams); err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to create virtual machine cluster",
			Error:   err.Error(),
		})
		return
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    createRequest,
		Message: "created virtual machine cluster successfully",
	})
}

// DeleteCluster handles POST /delete/cluster requests to delete multiple VMs
func (h *VirtualMachine) DeleteCluster(writer http.ResponseWriter, request *http.Request) {
	var deleteRequest contracts.DeleteClusterRequest
	cb, err := parseBodyAndHandleError(writer, request, &deleteRequest, true)
	if err != nil {
		cb()
		return
	}

	if len(deleteRequest.VirtualMachines) == 0 {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "no virtual machines specified in request",
		})
		return
	}

	// Adapt API contract to service params
	vmParams := h.spAdapter.AdaptDeleteCluster(deleteRequest)

	ctx := request.Context()
	if err := h.vmService.DeleteCluster(ctx, vmParams); err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to delete virtual machine cluster",
			Error:   err.Error(),
		})
		return
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    deleteRequest,
		Message: "deleted virtual machine cluster successfully",
	})
}

// StartCluster handles POST /start/cluster requests to start multiple VMs
func (h *VirtualMachine) StartCluster(writer http.ResponseWriter, request *http.Request) {
	var startRequest contracts.StartClusterRequest
	cb, err := parseBodyAndHandleError(writer, request, &startRequest, true)
	if err != nil {
		cb()
		return
	}

	if len(startRequest.VirtualMachines) == 0 {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "no virtual machines specified in request",
		})
		return
	}

	// Adapt API contract to service params
	vmParams := h.spAdapter.AdaptStartCluster(startRequest)

	ctx := request.Context()
	if err := h.vmService.StartCluster(ctx, vmParams); err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to start virtual machine cluster",
			Error:   err.Error(),
		})
		return
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    startRequest,
		Message: "started virtual machine cluster successfully",
	})
}

// QueryCluster handles GET /query/cluster requests to query VM information
func (h *VirtualMachine) QueryCluster(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	// Check if specific VMs are requested via query parameter or body
	var vmParams []parameters.QueryVM
	var queryRequest contracts.QueryClusterRequest

	// Try to parse body if present (for POST requests)
	if request.Method == http.MethodPost {
		cb, err := parseBodyAndHandleError(writer, request, &queryRequest, false)
		if err != nil {
			cb()
			return
		}
		if len(queryRequest.VirtualMachines) > 0 {
			vmParams = h.spAdapter.AdaptQueryCluster(queryRequest)
		}
	}

	// Query the service
	vmInfos, err := h.vmService.QueryCluster(ctx, vmParams)
	if err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to query virtual machines",
			Error:   err.Error(),
		})
		return
	}

	// Convert service VMInfo to API VMInfo
	response := contracts.QueryClusterResponse{
		VirtualMachines: h.spAdapter.AdaptVMInfoToAPI(vmInfos),
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    response,
		Message: "queried virtual machines successfully",
	})
}
