package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/infrastructure/cloudinit"
	"github.com/terabiome/homonculus/internal/infrastructure/disk"
	"github.com/terabiome/homonculus/internal/infrastructure/libvirt"
	"github.com/terabiome/homonculus/internal/runtime"
	"github.com/terabiome/homonculus/pkg/constants"
	"github.com/terabiome/homonculus/pkg/executor/fileops"
	pkglibvirt "github.com/terabiome/homonculus/pkg/libvirt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// VMService provides transport-agnostic VM operations.
type VMService struct {
	diskManager      *disk.Manager
	cloudinitManager *cloudinit.Manager
	libvirtManager   *libvirt.Manager
	connManager      *pkglibvirt.ConnectionManager
	logger           *slog.Logger

	vmDeleteCounter  metric.Int64Counter
	vmCloneCounter   metric.Int64Counter
	vmCreateDuration metric.Float64Histogram
	vmDeleteDuration metric.Float64Histogram
	vmCloneDuration  metric.Float64Histogram
}

// NewVMService creates a new VMService.
func NewVMService(
	diskManager *disk.Manager,
	cloudinitManager *cloudinit.Manager,
	libvirtManager *libvirt.Manager,
	connManager *pkglibvirt.ConnectionManager,
	logger *slog.Logger,
) *VMService {
	meter := otel.Meter("homonculus/service")

	vmDeleteCounter, err := meter.Int64Counter(
		"homonculus.vm.delete",
		metric.WithDescription("Number of VM delete operations"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		logger.Warn("failed to create vmDeleteCounter metric", slog.String("error", err.Error()))
	}

	vmCloneCounter, err := meter.Int64Counter(
		"homonculus.vm.clone",
		metric.WithDescription("Number of VM clone operations"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		logger.Warn("failed to create vmCloneCounter metric", slog.String("error", err.Error()))
	}

	vmCreateDuration, err := meter.Float64Histogram(
		"homonculus.vm.create.duration",
		metric.WithDescription("Duration of VM create operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		logger.Warn("failed to create vmCreateDuration metric", slog.String("error", err.Error()))
	}

	vmDeleteDuration, err := meter.Float64Histogram(
		"homonculus.vm.delete.duration",
		metric.WithDescription("Duration of VM delete operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		logger.Warn("failed to create vmDeleteDuration metric", slog.String("error", err.Error()))
	}

	vmCloneDuration, err := meter.Float64Histogram(
		"homonculus.vm.clone.duration",
		metric.WithDescription("Duration of VM clone operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		logger.Warn("failed to create vmCloneDuration metric", slog.String("error", err.Error()))
	}

	return &VMService{
		diskManager:      diskManager,
		cloudinitManager: cloudinitManager,
		libvirtManager:   libvirtManager,
		connManager:      connManager,
		logger:           logger.With(slog.String("service", "vm")),
		vmDeleteCounter:  vmDeleteCounter,
		vmCloneCounter:   vmCloneCounter,
		vmCreateDuration: vmCreateDuration,
		vmDeleteDuration: vmDeleteDuration,
		vmCloneDuration:  vmCloneDuration,
	}
}

// CreateCluster creates multiple VMs from transport-agnostic parameters.
func (s *VMService) CreateCluster(ctx context.Context, vms []CreateVMParams) error {
	tracer := otel.Tracer("homonculus/service")
	ctx, span := tracer.Start(ctx, "CreateCluster")
	defer span.End()

	span.SetAttributes(attribute.Int("vm.count", len(vms)))

	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := runtime.HypervisorContext{
		URI:      "qemu:///system",
		Conn:     conn,
		Executor: exec,
	}

	var failedVMs []string

	for _, vm := range vms {
		startTime := time.Now()
		_, vmSpan := tracer.Start(ctx, "CreateVM")
		vmSpan.SetAttributes(attribute.String("vm.name", vm.Name))

		virtualMachineUUID := uuid.New()

		exists, err := s.libvirtManager.CheckVirtualMachineExistence(vm.Name)
		if err != nil {
			s.logger.Error("failed to check if VM exists",
				slog.String("vm", vm.Name),
				slog.String("error", err.Error()),
			)
			vmSpan.End()
			failedVMs = append(failedVMs, vm.Name)
			continue
		}

		if exists {
			s.logger.Warn("VM already exists, skipping",
				slog.String("vm", vm.Name),
			)
			vmSpan.End()
			continue
		}

		s.logger.Info("creating VM disk",
			slog.String("vm", vm.Name),
			slog.String("uuid", virtualMachineUUID.String()),
			slog.String("path", vm.DiskPath),
			slog.Int64("size_gb", vm.DiskSizeGB),
		)

		// Convert service params to infrastructure contract
		createReq := s.toCreateVMRequest(vm)

		if err := s.diskManager.CreateDisk(ctx, hypervisor, createReq); err != nil {
			s.logger.Error("failed to create disk",
				slog.String("vm", vm.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			vmSpan.End()
			failedVMs = append(failedVMs, vm.Name)
			continue
		}

		if vm.CloudInitISOPath != "" {
			if err := s.cloudinitManager.CreateISO(ctx, hypervisor, createReq, virtualMachineUUID); err != nil {
				s.logger.Error("failed to create cloud-init ISO",
					slog.String("vm", vm.Name),
					slog.String("uuid", virtualMachineUUID.String()),
					slog.String("error", err.Error()),
				)
				if err := fileops.RemoveFile(ctx, hypervisor.Executor, vm.DiskPath); err != nil {
					s.logger.Warn("failed to cleanup disk",
						slog.String("path", vm.DiskPath),
						slog.String("error", err.Error()),
					)
				}
				vmSpan.End()
				failedVMs = append(failedVMs, vm.Name)
				continue
			}
		} else {
			s.logger.Debug("skipping cloud-init ISO creation", slog.String("vm", vm.Name))
		}

		if err := s.libvirtManager.CreateVirtualMachine(ctx, hypervisor, createReq, virtualMachineUUID); err != nil {
			s.logger.Error("failed to create VM",
				slog.String("vm", vm.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			if err := fileops.RemoveFile(ctx, hypervisor.Executor, vm.DiskPath); err != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", vm.DiskPath),
					slog.String("error", err.Error()),
				)
			}
			if vm.CloudInitISOPath != "" {
				if err := fileops.RemoveFile(ctx, hypervisor.Executor, vm.CloudInitISOPath); err != nil {
					s.logger.Warn("failed to cleanup cloud-init ISO",
						slog.String("path", vm.CloudInitISOPath),
						slog.String("error", err.Error()),
					)
				}
			}
			vmSpan.End()
			failedVMs = append(failedVMs, vm.Name)
			continue
		}

		s.logger.Info("successfully created VM",
			slog.String("vm", vm.Name),
			slog.String("uuid", virtualMachineUUID.String()),
		)
		if s.vmCreateDuration != nil {
			s.vmCreateDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("vm.name", vm.Name),
			))
		}
		vmSpan.End()
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to create %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

// DeleteCluster deletes multiple VMs.
func (s *VMService) DeleteCluster(ctx context.Context, vms []DeleteVMParams) error {
	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := runtime.HypervisorContext{
		URI:      "qemu:///system",
		Conn:     conn,
		Executor: exec,
	}

	var failedVMs []string

	for _, vm := range vms {
		startTime := time.Now()
		s.logger.Info("deleting VM", slog.String("vm", vm.Name))

		// Convert service params to infrastructure contract
		deleteReq := s.toDeleteVMRequest(vm)

		if vmUUID, err := s.libvirtManager.DeleteVirtualMachine(ctx, hypervisor, deleteReq); err != nil {
			s.logger.Error("failed to delete VM",
				slog.String("vm", vm.Name),
				slog.String("uuid", vmUUID),
				slog.String("error", err.Error()),
			)
			if s.vmDeleteCounter != nil {
				s.vmDeleteCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("status", "failed"),
				))
			}
			failedVMs = append(failedVMs, vm.Name)
			continue
		}

		s.logger.Info("successfully deleted VM", slog.String("vm", vm.Name))
		if s.vmDeleteCounter != nil {
			s.vmDeleteCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "success"),
			))
		}
		if s.vmDeleteDuration != nil {
			s.vmDeleteDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("vm.name", vm.Name),
			))
		}
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to delete %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

// CloneCluster clones a base VM into multiple target VMs.
func (s *VMService) CloneCluster(ctx context.Context, params CloneVMParams) error {
	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := runtime.HypervisorContext{
		URI:      "qemu:///system",
		Conn:     conn,
		Executor: exec,
	}

	s.logger.Info("finding base VM for cloning", slog.String("base_vm", params.BaseVMName))

	baseDomain, err := s.libvirtManager.FindVirtualMachine(params.BaseVMName)
	if err != nil {
		s.logger.Error("failed to find base VM",
			slog.String("base_vm", params.BaseVMName),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("unable to find base virtual machine %v: %w", params.BaseVMName, err)
	}

	baseDomainXML, err := s.libvirtManager.ToLibvirtXML(baseDomain)
	if err != nil {
		s.logger.Error("failed to get base VM XML",
			slog.String("base_vm", params.BaseVMName),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("unable to get XML for base virtual machine %v: %w", params.BaseVMName, err)
	}

	var baseImagePath string
	for _, disk := range baseDomainXML.Devices.Disks {
		if disk.Driver.Type == "qcow2" {
			baseImagePath = disk.Source.File.File
			break
		}
	}

	s.logger.Debug("found base image", slog.String("path", baseImagePath))

	var failedVMs []string

	for _, targetVM := range params.TargetSpecs {
		startTime := time.Now()
		virtualMachineUUID := uuid.New()

		s.logger.Info("cloning VM",
			slog.String("vm", targetVM.Name),
			slog.String("uuid", virtualMachineUUID.String()),
			slog.String("from", params.BaseVMName),
		)

		// Set base image path from base VM
		targetVM.BaseImagePath = baseImagePath

		// Convert service params to infrastructure contract
		targetSpec := s.toTargetVMSpec(targetVM)

		if err := s.diskManager.CreateDiskForClone(ctx, hypervisor, targetSpec); err != nil {
			s.logger.Error("failed to clone disk",
				slog.String("vm", targetVM.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			if err := fileops.RemoveFile(ctx, hypervisor.Executor, targetVM.DiskPath); err != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", targetVM.DiskPath),
					slog.String("error", err.Error()),
				)
			}
			if s.vmCloneCounter != nil {
				s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("status", "failed"),
					attribute.String("reason", "disk_clone_error"),
				))
			}
			failedVMs = append(failedVMs, targetVM.Name)
			continue
		}

		if err := s.libvirtManager.CloneVirtualMachine(ctx, hypervisor, baseDomainXML, targetSpec, virtualMachineUUID); err != nil {
			s.logger.Error("failed to clone VM",
				slog.String("vm", targetVM.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			if err := fileops.RemoveFile(ctx, hypervisor.Executor, targetVM.DiskPath); err != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", targetVM.DiskPath),
					slog.String("error", err.Error()),
				)
			}
			if s.vmCloneCounter != nil {
				s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("status", "failed"),
					attribute.String("reason", "vm_clone_error"),
				))
			}
			failedVMs = append(failedVMs, targetVM.Name)
			continue
		}

		s.logger.Info("successfully cloned VM",
			slog.String("vm", targetVM.Name),
			slog.String("uuid", virtualMachineUUID.String()),
		)
		if s.vmCloneCounter != nil {
			s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "success"),
			))
		}
		if s.vmCloneDuration != nil {
			s.vmCloneDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("vm.name", targetVM.Name),
			))
		}
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to clone %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

// Helper methods to convert service params to infrastructure contracts

func (s *VMService) toCreateVMRequest(params CreateVMParams) api.CreateVMRequest {
	// Convert service.UserConfig to api.UserConfig
	userConfigs := make([]api.UserConfig, len(params.UserConfigs))
	for i, uc := range params.UserConfigs {
		userConfigs[i] = api.UserConfig{
			Username:          uc.Username,
			SSHAuthorizedKeys: uc.SSHAuthorizedKeys,
		}
	}

	return api.CreateVMRequest{
		Name:                   params.Name,
		VCPU:                   params.VCPU,
		MemoryMB:               params.MemoryMB,
		DiskPath:               params.DiskPath,
		DiskSizeGB:             params.DiskSizeGB,
		BaseImagePath:          params.BaseImagePath,
		BridgeNetworkInterface: params.BridgeNetworkInterface,
		CloudInitISOPath:       params.CloudInitISOPath,
		Role:                   constants.KubernetesRole(params.Role),
		UserConfigs:            userConfigs,
	}
}

func (s *VMService) toDeleteVMRequest(params DeleteVMParams) api.DeleteVMRequest {
	return api.DeleteVMRequest{
		Name: params.Name,
	}
}

func (s *VMService) toTargetVMSpec(spec TargetVMSpec) api.TargetVMSpec {
	return api.TargetVMSpec{
		Name:          spec.Name,
		VCPU:          spec.VCPU,
		MemoryMB:      spec.MemoryMB,
		DiskPath:      spec.DiskPath,
		DiskSizeGB:    spec.DiskSizeGB,
		BaseImagePath: spec.BaseImagePath,
	}
}

