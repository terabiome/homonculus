package libvirt

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/terabiome/homonculus/pkg/executor"
	"libvirt.org/go/libvirt"
)

type ConnectionManager struct {
	conn     *libvirt.Connect
	executor executor.Executor
	mu       sync.Mutex
	logger   *slog.Logger
}

func NewConnectionManager(logger *slog.Logger) (*ConnectionManager, error) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %w", err)
	}

	logger.Info("libvirt connection established", slog.String("uri", "qemu:///system"))

	return &ConnectionManager{
		conn:     conn,
		executor: executor.NewLocal(logger),
		logger:   logger,
	}, nil
}

func (cm *ConnectionManager) GetHypervisor() (*libvirt.Connect, executor.Executor, func(), error) {
	cm.mu.Lock()

	alive, err := cm.conn.IsAlive()
	if err != nil || !alive {
		cm.logger.Warn("connection unhealthy, attempting reconnect")
		if err := cm.reconnect(); err != nil {
			cm.mu.Unlock()
			return nil, nil, nil, err
		}
	}

	unlock := func() { cm.mu.Unlock() }
	return cm.conn, cm.executor, unlock, nil
}

func (cm *ConnectionManager) reconnect() error {
	if cm.conn != nil {
		cm.conn.Close()
	}

	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return fmt.Errorf("reconnection failed: %w", err)
	}

	cm.conn = conn
	cm.logger.Info("libvirt reconnected")
	return nil
}

func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.conn != nil {
		cm.logger.Info("closing libvirt connection")
		_, err := cm.conn.Close()
		return err
	}
	return nil
}

