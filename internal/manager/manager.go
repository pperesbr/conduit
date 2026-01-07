package manager

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pperesbr/conduit/internal/config"
	"github.com/pperesbr/gokit/pkg/tunnel"
)

// HealthStatus represents the health and status information for a specific tunnel.
type HealthStatus struct {
	Name    string
	Status  tunnel.Status
	Error   error
	Healthy bool
}

// Manager manages SSH tunnels, their configurations, and controls their lifecycle, including start, stop, and restart.
type Manager struct {
	sshConfig   *tunnel.SSHConfig
	tunnels     map[string]*tunnel.Tunnel
	configs     map[string]config.TunnelConfig
	tunnelDones map[string]chan struct{}
	done        chan struct{}
	mu          sync.RWMutex
}

// NewManager initializes and returns a new instance of Manager to manage SSH tunnels and their configurations.
func NewManager(sshConfig *tunnel.SSHConfig) *Manager {
	return &Manager{
		sshConfig:   sshConfig,
		tunnels:     make(map[string]*tunnel.Tunnel),
		configs:     make(map[string]config.TunnelConfig),
		tunnelDones: make(map[string]chan struct{}),
		done:        make(chan struct{}),
	}
}

// Add registers a new tunnel configuration and initializes the associated SSH tunnel if the name is not already in use.
func (m *Manager) Add(cfg config.TunnelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tunnels[cfg.Name]; exists {
		return fmt.Errorf("tunnel %s already exists", cfg.Name)
	}

	tun := tunnel.NewTunnel(m.sshConfig, cfg.RemoteHost, cfg.RemotePort, cfg.LocalPort)
	m.tunnels[cfg.Name] = tun
	m.configs[cfg.Name] = cfg

	return nil
}

// Remove stops and removes the specified tunnel by name, along with its configuration, if it exists.
func (m *Manager) Remove(name string) error {
	m.stopAutoRestartForTunnel(name)

	m.mu.Lock()
	defer m.mu.Unlock()

	tun, exists := m.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel %s not found", name)
	}

	if tun.Status() == tunnel.StatusRunning {
		if err := tun.Stop(); err != nil {
			return fmt.Errorf("failed to stop tunnel %s: %w", name, err)
		}
	}

	delete(m.tunnels, name)
	delete(m.configs, name)

	return nil
}

// Start attempts to start the tunnel identified by the given name, returning an error if it fails or doesn't exist.
func (m *Manager) Start(name string) error {
	m.mu.RLock()
	tun, exists := m.tunnels[name]
	cfg, _ := m.configs[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("tunnel %s not found", name)
	}

	if err := tun.Start(); err != nil {
		return fmt.Errorf("failed to start tunnel %s: %w", name, err)
	}

	if cfg.AutoRestart.Enabled {
		m.startAutoRestartForTunnel(name, cfg.AutoRestart.Interval)
	}

	return nil
}

// Stop halts the tunnel identified by the given name, ensuring it is no longer active. Returns an error if unsuccessful.
func (m *Manager) Stop(name string) error {
	m.stopAutoRestartForTunnel(name)

	m.mu.RLock()
	tun, exists := m.tunnels[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("tunnel %s not found", name)
	}

	if err := tun.Stop(); err != nil {
		return fmt.Errorf("failed to stop tunnel %s: %w", name, err)
	}

	return nil
}

// Restart attempts to restart the tunnel identified by the given name, returning an error if the tunnel doesn't exist or fails to restart.
func (m *Manager) Restart(name string) error {
	m.mu.RLock()
	tun, exists := m.tunnels[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("tunnel %s not found", name)
	}

	if err := tun.Restart(); err != nil {
		return fmt.Errorf("failed to restart tunnel %s: %w", name, err)
	}

	return nil
}

// StartAll starts all registered SSH tunnels, returning a map of tunnel names to errors for any failures encountered.
func (m *Manager) StartAll() map[string]error {
	m.mu.RLock()
	names := make([]string, 0, len(m.tunnels))
	for name := range m.tunnels {
		names = append(names, name)
	}
	m.mu.RUnlock()

	errors := make(map[string]error)
	for _, name := range names {
		if err := m.Start(name); err != nil {
			errors[name] = err
		}
	}

	return errors
}

// StopAll stops all active tunnels managed by the Manager and returns a map of tunnel names to their associated stop errors.
func (m *Manager) StopAll() map[string]error {
	m.mu.Lock()
	for name, done := range m.tunnelDones {
		close(done)
		delete(m.tunnelDones, name)
	}
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	errors := make(map[string]error)
	for name, tun := range m.tunnels {
		if err := tun.Stop(); err != nil {
			errors[name] = err
		}
	}

	return errors
}

// Get returns the tunnel associated with the given name or nil if no such tunnel exists.
func (m *Manager) Get(name string) *tunnel.Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.tunnels[name]
}

// List returns a slice of strings containing the names of all registered SSH tunnels managed by the Manager.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.tunnels))
	for name := range m.tunnels {
		names = append(names, name)
	}

	return names
}

// Status returns the status of all managed tunnels as a map of tunnel names to their current statuses.
func (m *Manager) Status() map[string]tunnel.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]tunnel.Status)
	for name, tun := range m.tunnels {
		status[name] = tun.Status()
	}

	return status
}

// Stats retrieves statistics for all managed tunnels as a map of tunnel names to their respective stats.
func (m *Manager) Stats() map[string]tunnel.Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]tunnel.Stats)
	for name, tun := range m.tunnels {
		stats[name] = tun.Stats()
	}

	return stats
}

// HealthCheck evaluates the health status of all managed tunnels and returns a slice of their HealthStatus.
func (m *Manager) HealthCheck() []HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]HealthStatus, 0, len(m.tunnels))

	for name, tun := range m.tunnels {
		status := tun.Status()
		lastErr := tun.LastError()
		healthy := status == tunnel.StatusRunning && lastErr == nil

		results = append(results, HealthStatus{
			Name:    name,
			Status:  status,
			Error:   lastErr,
			Healthy: healthy,
		})
	}

	return results
}

// Unhealthy returns a slice of HealthStatus objects representing tunnels that are not in a healthy state.
func (m *Manager) Unhealthy() []HealthStatus {
	all := m.HealthCheck()

	unhealthy := make([]HealthStatus, 0)
	for _, h := range all {
		if !h.Healthy {
			unhealthy = append(unhealthy, h)
		}
	}

	return unhealthy
}

// Reconcile updates the Manager's state to match the provided configuration, modifying tunnel configurations as needed.
func (m *Manager) Reconcile(newConfig *config.Config) error {
	m.mu.Lock()
	m.sshConfig = &newConfig.SSH
	m.mu.Unlock()

	currentNames := make(map[string]bool)
	for _, name := range m.List() {
		currentNames[name] = true
	}

	newNames := make(map[string]bool)
	newConfigs := make(map[string]config.TunnelConfig)
	for _, cfg := range newConfig.TunnelConfigs {
		newNames[cfg.Name] = true
		newConfigs[cfg.Name] = cfg
	}

	for name := range currentNames {
		if !newNames[name] {
			log.Printf("reconcile: removing tunnel %s", name)
			if err := m.Remove(name); err != nil {
				log.Printf("reconcile: failed to remove %s: %v", name, err)
			}
		}
	}

	for name, cfg := range newConfigs {
		if !currentNames[name] {
			log.Printf("reconcile: adding tunnel %s", name)
			if err := m.Add(cfg); err != nil {
				log.Printf("reconcile: failed to add %s: %v", name, err)
				continue
			}
			if err := m.Start(name); err != nil {
				log.Printf("reconcile: failed to start %s: %v", name, err)
			}
		}
	}

	for name, newCfg := range newConfigs {
		if currentNames[name] {
			m.mu.RLock()
			oldCfg, exists := m.configs[name]
			m.mu.RUnlock()

			if exists && tunnelConfigChanged(oldCfg, newCfg) {
				log.Printf("reconcile: tunnel %s changed, restarting", name)

				m.mu.Lock()
				m.configs[name] = newCfg
				m.mu.Unlock()

				if err := m.Restart(name); err != nil {
					log.Printf("reconcile: failed to restart %s: %v", name, err)
				}
			}
		}
	}

	return nil
}

// Close terminates the Manager, stops all tunnels, and releases resources. Returns an error if any tunnel fails to stop.
func (m *Manager) Close() error {
	close(m.done)
	errors := m.StopAll()

	if len(errors) > 0 {
		return fmt.Errorf("errors closing manager: %v", errors)
	}

	return nil
}

// startAutoRestartForTunnel initiates a periodic restart mechanism for the specified tunnel based on the given interval.
func (m *Manager) startAutoRestartForTunnel(name string, interval time.Duration) {
	m.mu.Lock()
	if done, exists := m.tunnelDones[name]; exists {
		close(done)
	}

	done := make(chan struct{})
	m.tunnelDones[name] = done
	m.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.mu.RLock()
				tun, exists := m.tunnels[name]
				m.mu.RUnlock()

				if !exists {
					return
				}

				status := tun.Status()
				lastErr := tun.LastError()
				if status == tunnel.StatusError || lastErr != nil {
					_ = m.Restart(name)
				}
			case <-done:
				return
			case <-m.done:
				return
			}
		}
	}()
}

// stopAutoRestartForTunnel stops the auto-restart mechanism for the tunnel identified by the given name, if it exists.
func (m *Manager) stopAutoRestartForTunnel(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if done, exists := m.tunnelDones[name]; exists {
		close(done)
		delete(m.tunnelDones, name)
	}
}

// tunnelConfigChanged checks if there are any differences between the old and new TunnelConfig structures.
func tunnelConfigChanged(old, new config.TunnelConfig) bool {
	if old.RemoteHost != new.RemoteHost {
		return true
	}
	if old.RemotePort != new.RemotePort {
		return true
	}
	if old.LocalPort != new.LocalPort {
		return true
	}
	if old.AutoRestart.Enabled != new.AutoRestart.Enabled {
		return true
	}
	if old.AutoRestart.Interval != new.AutoRestart.Interval {
		return true
	}
	return false
}
