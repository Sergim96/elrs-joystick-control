package headtrackerbt

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Device struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

func Scan(ctx context.Context, duration time.Duration) ([]Device, error) {
	if duration <= 0 {
		duration = 5 * time.Second
	}

	if _, err := exec.LookPath("bluetoothctl"); err != nil {
		return nil, errors.New("bluetoothctl not found in PATH")
	}

	timeoutSeconds := int(duration.Round(time.Second) / time.Second)
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}

	scanCmd := exec.CommandContext(ctx, "bluetoothctl", "--timeout", fmt.Sprintf("%d", timeoutSeconds), "scan", "on")
	if out, err := scanCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("bluetooth scan failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	// Ensure scan is disabled after discovery; leaving scan active can interfere
	// with stable BLE GATT connections on some adapters.
	_ = exec.CommandContext(ctx, "bluetoothctl", "scan", "off").Run()

	devicesCmd := exec.CommandContext(ctx, "bluetoothctl", "devices")
	out, err := devicesCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing bluetooth devices failed: %w", err)
	}

	devices := parseBluetoothctlDevices(out)
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Name == devices[j].Name {
			return devices[i].Address < devices[j].Address
		}
		return devices[i].Name < devices[j].Name
	})

	return devices, nil
}

func parseBluetoothctlDevices(out []byte) []Device {
	var devices []Device
	seen := make(map[string]struct{})

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Device ") {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}

		address := strings.TrimSpace(parts[1])
		name := ""
		if len(parts) == 3 {
			name = strings.TrimSpace(parts[2])
		}

		if address == "" {
			continue
		}
		if _, ok := seen[address]; ok {
			continue
		}
		seen[address] = struct{}{}

		devices = append(devices, Device{
			Address: address,
			Name:    name,
		})
	}

	return devices
}
