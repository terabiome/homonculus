package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/dependencies"
	"github.com/terabiome/homonculus/internal/service/infrastructure/cloudinit"
	"github.com/terabiome/homonculus/internal/service/infrastructure/disk"
	"github.com/terabiome/homonculus/internal/service/infrastructure/libvirt"
	"github.com/terabiome/homonculus/internal/service/parameters"
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
func (s *VMService) CreateCluster(ctx context.Context, vms []parameters.CreateVM) error {
	tracer := otel.Tracer("homonculus/service")
	ctx, span := tracer.Start(ctx, "CreateCluster")
	defer span.End()

	span.SetAttributes(attribute.Int("vm.count", len(vms)))

	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := dependencies.HypervisorContext{
		URI:      s.connManager.GetURI(),
		Conn:     conn,
		Executor: exec,
	}

	var failedVMs []string

	for _, vm := range vms {
		startTime := time.Now()
		_, vmSpan := tracer.Start(ctx, "CreateVM")
		vmSpan.SetAttributes(attribute.String("vm.name", vm.Name))

		virtualMachineUUID := uuid.New()

		exists, err := s.libvirtManager.CheckVirtualMachineExistence(hypervisor, vm.Name)
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

		if err := s.diskManager.CreateDisk(ctx, hypervisor, vm); err != nil {
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
			if err := s.cloudinitManager.CreateISO(ctx, hypervisor, vm, virtualMachineUUID); err != nil {
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

		if err := s.libvirtManager.CreateVirtualMachine(ctx, hypervisor, vm, virtualMachineUUID); err != nil {
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
func (s *VMService) DeleteCluster(ctx context.Context, vms []parameters.DeleteVM) error {
	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := dependencies.HypervisorContext{
		URI:      s.connManager.GetURI(),
		Conn:     conn,
		Executor: exec,
	}

	var failedVMs []string

	for _, vm := range vms {
		startTime := time.Now()
		s.logger.Info("deleting VM", slog.String("vm", vm.Name))

		if vmUUID, err := s.libvirtManager.DeleteVirtualMachine(ctx, hypervisor, vm); err != nil {
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

// StartCluster starts multiple VMs.
func (s *VMService) StartCluster(ctx context.Context, vms []parameters.StartVM) error {
	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := dependencies.HypervisorContext{
		URI:      s.connManager.GetURI(),
		Conn:     conn,
		Executor: exec,
	}

	var failedVMs []string

	for _, vm := range vms {
		s.logger.Info("starting VM", slog.String("vm", vm.Name))

		if err := s.libvirtManager.StartVirtualMachine(ctx, hypervisor, vm); err != nil {
			s.logger.Error("failed to start VM",
				slog.String("vm", vm.Name),
				slog.String("error", err.Error()),
			)
			failedVMs = append(failedVMs, vm.Name)
			continue
		}

		s.logger.Info("successfully started VM", slog.String("vm", vm.Name))
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to start %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

// QueryCluster queries information about multiple VMs.
// If vms is empty, it lists all VMs. Otherwise, it queries specific VMs.
func (s *VMService) QueryCluster(ctx context.Context, vms []parameters.QueryVM) ([]parameters.VMInfo, error) {
	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return nil, fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := dependencies.HypervisorContext{
		URI:      s.connManager.GetURI(),
		Conn:     conn,
		Executor: exec,
	}

	var vmInfos []parameters.VMInfo
	var failedVMs []string

	// If no VMs specified, list all VMs
	if len(vms) == 0 {
		s.logger.Debug("listing all VMs")

		apiVMInfos, err := s.libvirtManager.ListAllVirtualMachines(ctx, hypervisor)
		if err != nil {
			return nil, fmt.Errorf("failed to list VMs: %w", err)
		}

		s.logger.Info("listed all VMs", slog.Int("count", len(apiVMInfos)))
		return vmInfos, nil
	}

	// Query specific VMs
	for _, vm := range vms {
		s.logger.Debug("querying VM", slog.String("vm", vm.Name))

		apiVMInfo, err := s.libvirtManager.GetVirtualMachineInfo(ctx, hypervisor, vm)
		if err != nil {
			s.logger.Error("failed to query VM",
				slog.String("vm", vm.Name),
				slog.String("error", err.Error()),
			)
			failedVMs = append(failedVMs, vm.Name)
			continue
		}

		s.logger.Debug("successfully queried VM", slog.String("vm", apiVMInfo.Name), slog.String("state", apiVMInfo.State))
	}

	if len(failedVMs) > 0 {
		return vmInfos, fmt.Errorf("failed to query %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return vmInfos, nil
}
