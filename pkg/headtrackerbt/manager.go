package headtrackerbt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type State struct {
	Devices        []Device   `json:"devices"`
	Selected       *Device    `json:"selected,omitempty"`
	LastScanAt     *time.Time `json:"lastScanAt,omitempty"`
	LastScanError  string     `json:"lastScanError,omitempty"`
	ConnectMessage string     `json:"connectMessage,omitempty"`
}

type Manager struct {
	mu             sync.RWMutex
	devices        []Device
	selected       *Device
	lastScanAt     *time.Time
	lastScanError  string
	connectMessage string
	configPath     string
}

func NewManager() *Manager {
	return &Manager{
		configPath: "./headtracker-bt.json",
	}
}

func (m *Manager) Snapshot() State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := State{
		Devices:        append([]Device(nil), m.devices...),
		LastScanError:  m.lastScanError,
		ConnectMessage: m.connectMessage,
	}
	if m.selected != nil {
		sel := *m.selected
		state.Selected = &sel
	}
	if m.lastScanAt != nil {
		t := *m.lastScanAt
		state.LastScanAt = &t
	}
	return state
}

func (m *Manager) Selected() *Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.selected == nil {
		return nil
	}
	sel := *m.selected
	return &sel
}

func (m *Manager) Scan(ctx context.Context, duration time.Duration) State {
	devices, err := Scan(ctx, duration)
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastScanAt = &now
	if err != nil {
		m.lastScanError = err.Error()
		m.devices = nil
		m.connectMessage = ""
		return m.snapshotLocked()
	}

	m.lastScanError = ""
	m.devices = append([]Device(nil), devices...)

	if m.selected != nil {
		for _, d := range m.devices {
			if d.Address == m.selected.Address {
				sel := d
				m.selected = &sel
				break
			}
		}
	}

	return m.snapshotLocked()
}

func (m *Manager) Select(address string) (State, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.devices {
		if d.Address == address {
			sel := d
			m.selected = &sel
			if err := m.saveConfigLocked(sel.Address); err != nil {
				m.connectMessage = fmt.Sprintf("Selected, but could not save config: %s", err.Error())
			} else {
				m.connectMessage = "Selected for Bluetooth connection"
			}
			return m.snapshotLocked(), true
		}
	}

	m.connectMessage = ""
	return m.snapshotLocked(), false
}

func (m *Manager) SetConfigPath(path string) {
	if path == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configPath = path
}

func (m *Manager) AutoConnectFromConfig(ctx context.Context, timeout time.Duration) State {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	address, err := m.loadConfigAddress()
	if err != nil {
		m.mu.Lock()
		m.lastScanError = fmt.Sprintf("could not load bluetooth config: %s", err.Error())
		m.connectMessage = ""
		state := m.snapshotLocked()
		m.mu.Unlock()
		return state
	}
	if address == "" {
		return m.Snapshot()
	}

	state := m.Scan(ctx, timeout)
	if state.LastScanError != "" {
		return state
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.devices {
		if d.Address == address {
			sel := d
			m.selected = &sel
			m.connectMessage = "Auto-selected Bluetooth device from config"
			m.lastScanError = ""
			return m.snapshotLocked()
		}
	}

	m.connectMessage = ""
	m.lastScanError = fmt.Sprintf("saved Bluetooth device %s not available after %d seconds", address, int(timeout/time.Second))
	return m.snapshotLocked()
}

type savedConfig struct {
	Address string `json:"address"`
}

func (m *Manager) loadConfigAddress() (string, error) {
	m.mu.RLock()
	path := m.configPath
	m.mu.RUnlock()

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	var cfg savedConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", err
	}

	return cfg.Address, nil
}

func (m *Manager) saveConfigLocked(address string) error {
	cfg := savedConfig{Address: address}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(m.configPath, raw, 0644)
}

func (m *Manager) snapshotLocked() State {
	state := State{
		Devices:        append([]Device(nil), m.devices...),
		LastScanError:  m.lastScanError,
		ConnectMessage: m.connectMessage,
	}
	if m.selected != nil {
		sel := *m.selected
		state.Selected = &sel
	}
	if m.lastScanAt != nil {
		t := *m.lastScanAt
		state.LastScanAt = &t
	}
	return state
}

var DefaultManager = NewManager()
