package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/service"
)

// VirtualMachine handles VM-related HTTP requests
type VirtualMachine struct {
	vmService *service.VMService
	logger    *slog.Logger
}

// NewVirtualMachine creates a new VirtualMachine handler
func NewVirtualMachine(vmService *service.VMService, logger *slog.Logger) *VirtualMachine {
	return &VirtualMachine{
		vmService: vmService,
		logger:    logger,
	}
}

// CreateCluster handles POST /create/cluster requests to create multiple VMs
func (h *VirtualMachine) CreateCluster(writer http.ResponseWriter, request *http.Request) {
	var createRequest api.CreateClusterRequest
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
	vmParams := adaptCreateCluster(createRequest)

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
	var deleteRequest api.DeleteClusterRequest
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
	vmParams := adaptDeleteCluster(deleteRequest)

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
	var startRequest api.StartClusterRequest
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
	vmParams := adaptStartCluster(startRequest)

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
	var vmParams []service.QueryVMParams
	var queryRequest api.QueryClusterRequest

	// Try to parse body if present (for POST requests)
	if request.Method == http.MethodPost {
		cb, err := parseBodyAndHandleError(writer, request, &queryRequest, false)
		if err != nil {
			cb()
			return
		}
		if len(queryRequest.VirtualMachines) > 0 {
			vmParams = adaptQueryCluster(queryRequest)
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
	response := api.QueryClusterResponse{
		VirtualMachines: adaptVMInfoToAPI(vmInfos),
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    response,
		Message: "queried virtual machines successfully",
	})
}

// CloneCluster handles POST /clone/cluster requests to clone VMs
func (h *VirtualMachine) CloneCluster(writer http.ResponseWriter, request *http.Request) {
	var cloneRequest api.CloneClusterRequest
	cb, err := parseBodyAndHandleError(writer, request, &cloneRequest, true)
	if err != nil {
		cb()
		return
	}

	if len(cloneRequest.TargetVMs) == 0 {
		writeResult(writer, http.StatusBadRequest, GenericResponse{
			Body:    nil,
			Message: "no target virtual machines specified in request",
		})
		return
	}

	// Adapt API contract to service params
	cloneParams := adaptCloneCluster(cloneRequest)

	ctx := request.Context()
	if err := h.vmService.CloneCluster(ctx, cloneParams); err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    nil,
			Message: "failed to clone virtual machine cluster",
			Error:   err.Error(),
		})
		return
	}

	writeResult(writer, http.StatusOK, GenericResponse{
		Body:    cloneRequest,
		Message: "cloned virtual machine cluster successfully",
	})
}

// FormatRequest handles POST /format requests to format contract examples
func (h *VirtualMachine) FormatRequest(writer http.ResponseWriter, request *http.Request) {
	contractGeneratorMap := map[string]func() any{
		"create":       func() any { return api.CreateClusterRequest{} },
		"delete":       func() any { return api.DeleteClusterRequest{} },
		"start":        func() any { return api.StartClusterRequest{} },
		"query":        func() any { return api.QueryClusterRequest{} },
		"clone":        func() any { return api.CloneClusterRequest{} },
		"create_fleet": func() any { return api.CreateClusterRequest{} },
	}

	serializerMap := map[string]func(any) ([]byte, error){
		"json": func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") },
	}

	queries := request.URL.Query()

	var (
		inputData  any
		outputData []byte
		err        error
	)

	contractType := queries.Get("contract")
	if contractGenerator, ok := contractGeneratorMap[contractType]; !ok {
		writeResult(writer, http.StatusNotFound, GenericResponse{
			Body:    nil,
			Message: "no matching contract to format request",
		})
		return
	} else {
		inputData = contractGenerator()
	}

	cb, err := parseBodyAndHandleError(writer, request, &inputData, false)
	if err != nil {
		cb()
		return
	}

	formatType := queries.Get("format")
	if serializer, ok := serializerMap[formatType]; !ok {
		writeResult(writer, http.StatusNotFound, GenericResponse{
			Body:    nil,
			Message: "no matching serializer to format request",
		})
		return
	} else {
		outputData, err = serializer(inputData)
	}

	if err != nil {
		writeResult(writer, http.StatusInternalServerError, GenericResponse{
			Body:    err,
			Message: "could not serialize data",
		})
		return
	}

	writeBytes(writer, http.StatusOK, outputData)
}

// Adapter functions to convert API contracts to service params

func adaptCreateCluster(req api.CreateClusterRequest) []service.CreateVMParams {
	params := make([]service.CreateVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = adaptCreateVM(vm)
	}
	return params
}

func adaptCreateVM(vm api.CreateVMRequest) service.CreateVMParams {
	var tuning *service.VMTuning

	// Convert tuning configuration if present
	if vm.Tuning != nil {
		tuning = &service.VMTuning{
			VCPUPins:       vm.Tuning.VCPUPins,
			EmulatorCPUSet: vm.Tuning.EmulatorCPUSet,
		}

		// Convert NUMA memory if present
		if vm.Tuning.NUMAMemory != nil {
			tuning.NUMAMemory = &service.NUMAMemory{
				Nodeset: vm.Tuning.NUMAMemory.Nodeset,
				Mode:    vm.Tuning.NUMAMemory.Mode,
			}
		}
	}

	return service.CreateVMParams{
		Name:                   vm.Name,
		VCPUCount:              vm.VCPUCount,
		MemoryMB:               vm.MemoryMB,
		DiskPath:               vm.DiskPath,
		DiskSizeGB:             vm.DiskSizeGB,
		BaseImagePath:          vm.BaseImagePath,
		BridgeNetworkInterface: vm.BridgeNetworkInterface,
		CloudInitISOPath:       vm.CloudInitISOPath,
		Role:                   string(vm.Role),
		UserConfigs:            adaptUserConfigs(vm.UserConfigs),
		Runcmds:                vm.Runcmds,
		Tuning:                 tuning,
	}
}

func adaptDeleteCluster(req api.DeleteClusterRequest) []service.DeleteVMParams {
	params := make([]service.DeleteVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.DeleteVMParams{
			Name: vm.Name,
		}
	}
	return params
}

func adaptStartCluster(req api.StartClusterRequest) []service.StartVMParams {
	params := make([]service.StartVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.StartVMParams{
			Name: vm.Name,
		}
	}
	return params
}

func adaptQueryCluster(req api.QueryClusterRequest) []service.QueryVMParams {
	params := make([]service.QueryVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.QueryVMParams{
			Name: vm.Name,
		}
	}
	return params
}

func adaptVMInfoToAPI(vmInfos []service.VMInfo) []api.VMInfo {
	result := make([]api.VMInfo, len(vmInfos))
	for i, info := range vmInfos {
		disks := make([]api.DiskInfo, len(info.Disks))
		for j, d := range info.Disks {
			disks[j] = api.DiskInfo{
				Path:   d.Path,
				Type:   d.Type,
				Device: d.Device,
				SizeGB: d.SizeGB,
			}
		}
		result[i] = api.VMInfo{
			Name:       info.Name,
			UUID:       info.UUID,
			State:      info.State,
			VCPUCount:  info.VCPUCount,
			MemoryMB:   info.MemoryMB,
			Disks:      disks,
			AutoStart:  info.AutoStart,
			Persistent: info.Persistent,
			Hostname:   info.Hostname,
			IPAddress:  info.IPAddress,
		}
	}
	return result
}

func adaptCloneCluster(req api.CloneClusterRequest) service.CloneVMParams {
	targetSpecs := make([]service.TargetVMSpec, len(req.TargetVMs))
	for i, target := range req.TargetVMs {
		targetSpecs[i] = service.TargetVMSpec{
			Name:          target.Name,
			VCPUCount:     target.VCPUCount,
			MemoryMB:      target.MemoryMB,
			DiskPath:      target.DiskPath,
			DiskSizeGB:    target.DiskSizeGB,
			BaseImagePath: target.BaseImagePath,
		}
	}

	return service.CloneVMParams{
		BaseVMName:  req.BaseVM.Name,
		TargetSpecs: targetSpecs,
	}
}

func adaptUserConfigs(configs []api.UserConfig) []service.UserConfig {
	result := make([]service.UserConfig, len(configs))
	for i, c := range configs {
		result[i] = service.UserConfig{
			Username:          c.Username,
			SSHAuthorizedKeys: c.SSHAuthorizedKeys,
			Password:          c.Password,
		}
	}
	return result
}
