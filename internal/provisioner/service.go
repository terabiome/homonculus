package provisioner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/cloudinit"
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/internal/disk"
	"github.com/terabiome/homonculus/internal/libvirt"
	"github.com/terabiome/homonculus/pkg/executor"
	pkglibvirt "github.com/terabiome/homonculus/pkg/libvirt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Service struct {
	diskService      *disk.Service
	cloudinitService *cloudinit.Service
	libvirtService   *libvirt.Service
	connManager      *pkglibvirt.ConnectionManager
	logger           *slog.Logger

	vmDeleteCounter  metric.Int64Counter
	vmCloneCounter   metric.Int64Counter
	vmCreateDuration metric.Float64Histogram
	vmDeleteDuration metric.Float64Histogram
	vmCloneDuration  metric.Float64Histogram
}

func NewService(
	diskService *disk.Service,
	cloudinitService *cloudinit.Service,
	libvirtService *libvirt.Service,
	connManager *pkglibvirt.ConnectionManager,
	logger *slog.Logger,
) *Service {
	meter := otel.Meter("homonculus/provisioner")

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

	return &Service{
		diskService:      diskService,
		cloudinitService: cloudinitService,
		libvirtService:   libvirtService,
		connManager:      connManager,
		logger:           logger.With(slog.String("service", "provisioner")),
		vmDeleteCounter:  vmDeleteCounter,
		vmCloneCounter:   vmCloneCounter,
		vmCreateDuration: vmCreateDuration,
		vmDeleteDuration: vmDeleteDuration,
		vmCloneDuration:  vmCloneDuration,
	}
}

func (s *Service) CreateCluster(ctx context.Context, request contracts.CreateVirtualMachineClusterRequest) error {
	tracer := otel.Tracer("homonculus/provisioner")
	ctx, span := tracer.Start(ctx, "CreateCluster")
	defer span.End()

	span.SetAttributes(attribute.Int("vm.count", len(request.VirtualMachines)))

	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := contracts.HypervisorContext{
		URI:      "qemu:///system",
		Conn:     conn,
		Executor: exec,
	}

	var failedVMs []string

	for _, virtualMachine := range request.VirtualMachines {
		startTime := time.Now()
		_, vmSpan := tracer.Start(ctx, "CreateVM")
		vmSpan.SetAttributes(attribute.String("vm.name", virtualMachine.Name))

		virtualMachineUUID := uuid.New()

		exists, err := s.libvirtService.CheckVirtualMachineExistence(virtualMachine.Name)
		if err != nil {
			s.logger.Error("failed to check if VM exists",
				slog.String("vm", virtualMachine.Name),
				slog.String("error", err.Error()),
			)
			vmSpan.End()
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		if exists {
			s.logger.Warn("VM already exists, skipping",
				slog.String("vm", virtualMachine.Name),
			)
			vmSpan.End()
			continue
		}

		s.logger.Info("creating VM disk",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
			slog.String("path", virtualMachine.DiskPath),
			slog.Int64("size_gb", virtualMachine.DiskSizeGB),
		)

		if err := s.diskService.CreateDisk(ctx, hypervisor, virtualMachine); err != nil {
			s.logger.Error("failed to create disk",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			vmSpan.End()
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		if virtualMachine.CloudInitISOPath != "" {
			if err := s.cloudinitService.CreateISO(ctx, hypervisor, virtualMachine, virtualMachineUUID); err != nil {
				s.logger.Error("failed to create cloud-init ISO",
					slog.String("vm", virtualMachine.Name),
					slog.String("uuid", virtualMachineUUID.String()),
					slog.String("error", err.Error()),
				)
				result, cleanupErr := executor.RunAndCapture(ctx, hypervisor.Executor, "rm", "-f", virtualMachine.DiskPath)
				if cleanupErr != nil {
					s.logger.Warn("failed to cleanup disk",
						slog.String("path", virtualMachine.DiskPath),
						slog.String("error", cleanupErr.Error()),
						slog.String("stderr", result.Stderr),
					)
				}
				vmSpan.End()
				failedVMs = append(failedVMs, virtualMachine.Name)
				continue
			}
		} else {
			s.logger.Debug("skipping cloud-init ISO creation", slog.String("vm", virtualMachine.Name))
		}

		if err := s.libvirtService.CreateVirtualMachine(ctx, hypervisor, virtualMachine, virtualMachineUUID); err != nil {
			s.logger.Error("failed to create VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			result, cleanupErr := executor.RunAndCapture(ctx, hypervisor.Executor, "rm", "-f", virtualMachine.DiskPath)
			if cleanupErr != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", virtualMachine.DiskPath),
					slog.String("error", cleanupErr.Error()),
					slog.String("stderr", result.Stderr),
				)
			}
			if virtualMachine.CloudInitISOPath != "" {
				result, cleanupErr := executor.RunAndCapture(ctx, hypervisor.Executor, "rm", "-f", virtualMachine.CloudInitISOPath)
				if cleanupErr != nil {
					s.logger.Warn("failed to cleanup cloud-init ISO",
						slog.String("path", virtualMachine.CloudInitISOPath),
						slog.String("error", cleanupErr.Error()),
						slog.String("stderr", result.Stderr),
					)
				}
			}
			vmSpan.End()
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully created VM",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
		)
		if s.vmCreateDuration != nil {
			s.vmCreateDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("vm.name", virtualMachine.Name),
			))
		}
		vmSpan.End()
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to create %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) DeleteCluster(ctx context.Context, request contracts.DeleteVirtualMachineClusterRequest) error {
	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := contracts.HypervisorContext{
		URI:      "qemu:///system",
		Conn:     conn,
		Executor: exec,
	}

	var failedVMs []string

	for _, virtualMachine := range request.VirtualMachines {
		startTime := time.Now()
		s.logger.Info("deleting VM", slog.String("vm", virtualMachine.Name))

		if vmUUID, err := s.libvirtService.DeleteVirtualMachine(ctx, hypervisor, virtualMachine); err != nil {
			s.logger.Error("failed to delete VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", vmUUID),
				slog.String("error", err.Error()),
			)
			if s.vmDeleteCounter != nil {
				s.vmDeleteCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("status", "failed"),
				))
			}
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully deleted VM", slog.String("vm", virtualMachine.Name))
		if s.vmDeleteCounter != nil {
			s.vmDeleteCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "success"),
			))
		}
		if s.vmDeleteDuration != nil {
			s.vmDeleteDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("vm.name", virtualMachine.Name),
			))
		}
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to delete %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) CloneCluster(ctx context.Context, request contracts.CloneVirtualMachineClusterRequest) error {
	conn, exec, unlock, err := s.connManager.GetHypervisor()
	if err != nil {
		return fmt.Errorf("failed to get hypervisor connection: %w", err)
	}
	defer unlock()

	hypervisor := contracts.HypervisorContext{
		URI:      "qemu:///system",
		Conn:     conn,
		Executor: exec,
	}

	s.logger.Info("finding base VM for cloning", slog.String("base_vm", request.BaseVirtualMachine.Name))

	baseDomain, err := s.libvirtService.FindVirtualMachine(request.BaseVirtualMachine.Name)
	if err != nil {
		s.logger.Error("failed to find base VM",
			slog.String("base_vm", request.BaseVirtualMachine.Name),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("unable to find base virtual machine %v: %w", request.BaseVirtualMachine.Name, err)
	}

	baseDomainXML, err := s.libvirtService.ToLibvirtXML(baseDomain)
	if err != nil {
		s.logger.Error("failed to get base VM XML",
			slog.String("base_vm", request.BaseVirtualMachine.Name),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("unable to get XML for base virtual machine %v: %w", request.BaseVirtualMachine.Name, err)
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

	for _, virtualMachine := range request.TargetVirtualMachines {
		startTime := time.Now()
		virtualMachineUUID := uuid.New()

		s.logger.Info("cloning VM",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
			slog.String("from", request.BaseVirtualMachine.Name),
		)

		virtualMachine.BaseImagePath = baseImagePath

		if err := s.diskService.CreateDiskForClone(ctx, hypervisor, virtualMachine); err != nil {
			s.logger.Error("failed to clone disk",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			result, cleanupErr := executor.RunAndCapture(ctx, hypervisor.Executor, "rm", "-f", virtualMachine.DiskPath)
			if cleanupErr != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", virtualMachine.DiskPath),
					slog.String("error", cleanupErr.Error()),
					slog.String("stderr", result.Stderr),
				)
			}
			if s.vmCloneCounter != nil {
				s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("status", "failed"),
					attribute.String("reason", "disk_clone_error"),
				))
			}
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		if err := s.libvirtService.CloneVirtualMachine(ctx, hypervisor, baseDomainXML, virtualMachine, virtualMachineUUID); err != nil {
			s.logger.Error("failed to clone VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			result, cleanupErr := executor.RunAndCapture(ctx, hypervisor.Executor, "rm", "-f", virtualMachine.DiskPath)
			if cleanupErr != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", virtualMachine.DiskPath),
					slog.String("error", cleanupErr.Error()),
					slog.String("stderr", result.Stderr),
				)
			}
			if s.vmCloneCounter != nil {
				s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("status", "failed"),
					attribute.String("reason", "vm_clone_error"),
				))
			}
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully cloned VM",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
		)
		if s.vmCloneCounter != nil {
			s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "success"),
			))
		}
		if s.vmCloneDuration != nil {
			s.vmCloneDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("vm.name", virtualMachine.Name),
			))
		}
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to clone %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}
