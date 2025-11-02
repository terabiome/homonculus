package provisioner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/cloudinit"
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/internal/disk"
	"github.com/terabiome/homonculus/internal/libvirt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Service struct {
	diskService      *disk.Service
	cloudinitService *cloudinit.Service
	libvirtService   *libvirt.Service
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
	logger *slog.Logger,
) *Service {
	meter := otel.Meter("homonculus/provisioner")

	vmDeleteCounter, _ := meter.Int64Counter(
		"homonculus.vm.delete",
		metric.WithDescription("Number of VM delete operations"),
		metric.WithUnit("{operation}"),
	)

	vmCloneCounter, _ := meter.Int64Counter(
		"homonculus.vm.clone",
		metric.WithDescription("Number of VM clone operations"),
		metric.WithUnit("{operation}"),
	)

	vmCreateDuration, _ := meter.Float64Histogram(
		"homonculus.vm.create.duration",
		metric.WithDescription("Duration of VM create operations"),
		metric.WithUnit("s"),
	)

	vmDeleteDuration, _ := meter.Float64Histogram(
		"homonculus.vm.delete.duration",
		metric.WithDescription("Duration of VM delete operations"),
		metric.WithUnit("s"),
	)

	vmCloneDuration, _ := meter.Float64Histogram(
		"homonculus.vm.clone.duration",
		metric.WithDescription("Duration of VM clone operations"),
		metric.WithUnit("s"),
	)

	return &Service{
		diskService:      diskService,
		cloudinitService: cloudinitService,
		libvirtService:   libvirtService,
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

		if err := s.diskService.CreateDisk(virtualMachine.DiskPath, virtualMachine.BaseImagePath, virtualMachine.DiskSizeGB); err != nil {
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
			if err := s.cloudinitService.CreateISO(virtualMachine, virtualMachineUUID); err != nil {
				s.logger.Error("failed to create cloud-init ISO",
					slog.String("vm", virtualMachine.Name),
					slog.String("uuid", virtualMachineUUID.String()),
					slog.String("error", err.Error()),
				)
				if err := os.Remove(virtualMachine.DiskPath); err != nil {
					s.logger.Warn("failed to cleanup disk",
						slog.String("path", virtualMachine.DiskPath),
						slog.String("error", err.Error()),
					)
				}
				vmSpan.End()
				failedVMs = append(failedVMs, virtualMachine.Name)
				continue
			}
		} else {
			s.logger.Debug("skipping cloud-init ISO creation", slog.String("vm", virtualMachine.Name))
		}

		if err := s.libvirtService.CreateVirtualMachine(virtualMachine, virtualMachineUUID); err != nil {
			s.logger.Error("failed to create VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			if err := os.Remove(virtualMachine.DiskPath); err != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", virtualMachine.DiskPath),
					slog.String("error", err.Error()),
				)
			}
			if virtualMachine.CloudInitISOPath != "" {
				if err := os.Remove(virtualMachine.CloudInitISOPath); err != nil {
					s.logger.Warn("failed to cleanup cloud-init ISO",
						slog.String("path", virtualMachine.CloudInitISOPath),
						slog.String("error", err.Error()),
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
		s.vmCreateDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("vm.name", virtualMachine.Name),
		))
		vmSpan.End()
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to create %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) DeleteCluster(ctx context.Context, request contracts.DeleteVirtualMachineClusterRequest) error {
	var failedVMs []string

	for _, virtualMachine := range request.VirtualMachines {
		startTime := time.Now()
		s.logger.Info("deleting VM", slog.String("vm", virtualMachine.Name))

		if vmUUID, err := s.libvirtService.DeleteVirtualMachine(virtualMachine); err != nil {
			s.logger.Error("failed to delete VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", vmUUID),
				slog.String("error", err.Error()),
			)
			s.vmDeleteCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "failed"),
			))
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully deleted VM", slog.String("vm", virtualMachine.Name))
		s.vmDeleteCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "success"),
		))
		s.vmDeleteDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("vm.name", virtualMachine.Name),
		))
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to delete %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) CloneCluster(ctx context.Context, request contracts.CloneVirtualMachineClusterRequest) error {
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

		if err := s.diskService.CreateDisk(virtualMachine.DiskPath, baseImagePath, virtualMachine.DiskSizeGB); err != nil {
			s.logger.Error("failed to clone disk",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			if err := os.Remove(virtualMachine.DiskPath); err != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", virtualMachine.DiskPath),
					slog.String("error", err.Error()),
				)
			}
			s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "failed"),
				attribute.String("reason", "disk_clone_error"),
			))
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		virtualMachine.BaseImagePath = baseImagePath

		if err := s.libvirtService.CloneVirtualMachine(baseDomainXML, virtualMachine, virtualMachineUUID); err != nil {
			s.logger.Error("failed to clone VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "failed"),
				attribute.String("reason", "vm_clone_error"),
			))
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully cloned VM",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
		)
		s.vmCloneCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "success"),
		))
		s.vmCloneDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("vm.name", virtualMachine.Name),
		))
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to clone %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}
