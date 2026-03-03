package headtrackerbt

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func PromptSelectDevice(r io.Reader, w io.Writer, devices []Device) (Device, error) {
	if len(devices) == 0 {
		return Device{}, fmt.Errorf("no bluetooth devices found")
	}

	fmt.Fprintln(w, "Bluetooth devices:")
	for i, d := range devices {
		name := d.Name
		if strings.TrimSpace(name) == "" {
			name = "<unknown>"
		}
		fmt.Fprintf(w, "  [%d] %s  %s\n", i+1, d.Address, name)
	}
	fmt.Fprint(w, "Choose device number or enter MAC address: ")

	reader := bufio.NewReader(r)
	input, err := reader.ReadString('\n')
	if err != nil && len(input) == 0 {
		return Device{}, err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return Device{}, fmt.Errorf("no selection provided")
	}

	if n, err := strconv.Atoi(input); err == nil {
		if n < 1 || n > len(devices) {
			return Device{}, fmt.Errorf("selection out of range: %d", n)
		}
		return devices[n-1], nil
	}

	for _, d := range devices {
		if strings.EqualFold(d.Address, input) {
			return d, nil
		}
	}

	return Device{}, fmt.Errorf("device not found: %s", input)
}
