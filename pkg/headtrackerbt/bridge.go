package headtrackerbt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	bluezService   = "org.bluez"
	frskyCharUUID  = "0000fff6-0000-1000-8000-00805f9b34fb"
	startStop      = 0x7E
	byteStuff      = 0x7D
	stuffMask      = 0x20
	btChannels     = 8
	connectTimeout = 8 * time.Second
	serviceTimeout = 10 * time.Second
	lookupInterval = 250 * time.Millisecond
	signalBufSize  = 32
	retryDelay     = 1200 * time.Millisecond
	readPollPeriod = 200 * time.Millisecond
	primeScanDelay = 1500 * time.Millisecond
)

func StreamChannels(ctx context.Context, address string, onFrame func([]int)) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect system bus failed: %w", err)
	}
	defer conn.Close()

	addr := normalizeAddress(address)
	if addr == "" {
		return errors.New("empty bluetooth address")
	}

	devPath, charPath, err := connectAndResolve(ctx, conn, addr)
	if err != nil {
		return err
	}
	fmt.Printf("headtracker-bt: connected via dbus device=%s char=%s\n", devPath, charPath)

	charObj := conn.Object(bluezService, charPath)
	devObj := conn.Object(bluezService, devPath)
	if callErr := charObj.Call("org.bluez.GattCharacteristic1.StartNotify", 0).Err; callErr != nil &&
		!isBlueZAlreadyNotifying(callErr) {
		return fmt.Errorf("start notify failed: %w", callErr)
	}
	fmt.Println("headtracker-bt: notify started")
	defer func() {
		_ = charObj.Call("org.bluez.GattCharacteristic1.StopNotify", 0).Err
		_ = devObj.Call("org.bluez.Device1.Disconnect", 0).Err
	}()

	sigCh := make(chan *dbus.Signal, signalBufSize)
	conn.Signal(sigCh)
	defer conn.RemoveSignal(sigCh)

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(charPath),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return fmt.Errorf("add dbus signal match failed: %w", err)
	}

	decoder := newFrSkyDecoder()
	pollTicker := time.NewTicker(readPollPeriod)
	defer pollTicker.Stop()
	var lastPolled []byte
	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-sigCh:
			if sig == nil || sig.Path != charPath || len(sig.Body) < 2 {
				continue
			}
			iface, ok := sig.Body[0].(string)
			if !ok || iface != "org.bluez.GattCharacteristic1" {
				continue
			}
			changed, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			v, ok := changed["Value"]
			if !ok {
				continue
			}
			bytes, ok := v.Value().([]byte)
			if !ok || len(bytes) == 0 {
				continue
			}
			fmt.Printf("headtracker-bt: notify value len=%d\n", len(bytes))
			for _, b := range bytes {
				if channels, frameOK := decoder.feedByte(b); frameOK {
					fmt.Printf("headtracker-bt decoded channels: %v\n", channels)
					onFrame(channels)
				}
			}
		case <-pollTicker.C:
			var value []byte
			if readErr := charObj.Call("org.bluez.GattCharacteristic1.ReadValue", 0, map[string]dbus.Variant{}).Store(&value); readErr != nil {
				continue
			}
			if len(value) == 0 || bytesEqual(value, lastPolled) {
				continue
			}
			lastPolled = append(lastPolled[:0], value...)
			fmt.Printf("headtracker-bt: polled value len=%d\n", len(value))
			for _, b := range value {
				if channels, frameOK := decoder.feedByte(b); frameOK {
					fmt.Printf("headtracker-bt decoded channels: %v\n", channels)
					onFrame(channels)
				}
			}
		}
	}
}

func connectAndResolve(ctx context.Context, conn *dbus.Conn, address string) (dbus.ObjectPath, dbus.ObjectPath, error) {
	var lastErr error
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return "", "", fmt.Errorf("bluetooth connect cancelled: %w (last error: %v)", ctx.Err(), lastErr)
			}
			return "", "", ctx.Err()
		default:
		}
		attempt++

		devPath, err := findDevicePath(conn, address)
		if err != nil {
			lastErr = err
			if adapterPath, aerr := findAnyAdapterPath(conn); aerr == nil {
				primeDiscovery(ctx, conn, adapterPath, primeScanDelay)
			}
			if attempt == 1 || attempt%5 == 0 {
				fmt.Printf("headtracker-bt: connect attempt %d, device not found yet: %v\n", attempt, err)
			}
			time.Sleep(retryDelay)
			continue
		}
		devObj := conn.Object(bluezService, devPath)
		_ = stopAdapterDiscovery(conn, devPath)
		_ = setDeviceTrusted(devObj)

		if callErr := devObj.Call("org.bluez.Device1.Connect", 0).Err; callErr != nil &&
			!isBlueZAlreadyConnected(callErr) {
			lastErr = callErr
			fmt.Printf("headtracker-bt: connect attempt %d failed: %v\n", attempt, callErr)
			if shouldRetryConnectError(callErr) {
				_ = devObj.Call("org.bluez.Device1.Disconnect", 0).Err
				time.Sleep(retryDelay)
				continue
			}
			return "", "", fmt.Errorf("device connect failed: %w", callErr)
		}
		fmt.Printf("headtracker-bt: connect attempt %d succeeded\n", attempt)

		waitCtx, waitCancel := context.WithTimeout(ctx, connectTimeout)
		err = waitDeviceReady(waitCtx, conn, devPath)
		waitCancel()
		if err != nil {
			lastErr = err
			fmt.Printf("headtracker-bt: waiting device ready failed: %v\n", err)
			time.Sleep(retryDelay)
			continue
		}

		charCtx, charCancel := context.WithTimeout(ctx, serviceTimeout)
		charPath, err := waitCharacteristicPath(charCtx, conn, devPath, frskyCharUUID)
		charCancel()
		if err != nil {
			lastErr = err
			fmt.Printf("headtracker-bt: waiting characteristic failed: %v\n", err)
			time.Sleep(retryDelay)
			continue
		}

		return devPath, charPath, nil
	}
}

func stopAdapterDiscovery(conn *dbus.Conn, devicePath dbus.ObjectPath) error {
	p := string(devicePath)
	idx := strings.Index(p, "/dev_")
	if idx <= 0 {
		return nil
	}
	adapterPath := dbus.ObjectPath(p[:idx])
	adapterObj := conn.Object(bluezService, adapterPath)
	return adapterObj.Call("org.bluez.Adapter1.StopDiscovery", 0).Err
}

func setDeviceTrusted(devObj dbus.BusObject) error {
	return devObj.Call(
		"org.freedesktop.DBus.Properties.Set",
		0,
		"org.bluez.Device1",
		"Trusted",
		dbus.MakeVariant(true),
	).Err
}

func findAnyAdapterPath(conn *dbus.Conn) (dbus.ObjectPath, error) {
	managed, err := managedObjects(conn)
	if err != nil {
		return "", err
	}
	for path, ifaces := range managed {
		if _, ok := ifaces["org.bluez.Adapter1"]; ok {
			return path, nil
		}
	}
	return "", errors.New("no bluetooth adapter found")
}

func primeDiscovery(ctx context.Context, conn *dbus.Conn, adapterPath dbus.ObjectPath, delay time.Duration) {
	adapterObj := conn.Object(bluezService, adapterPath)
	_ = adapterObj.Call("org.bluez.Adapter1.StartDiscovery", 0).Err
	select {
	case <-ctx.Done():
	case <-time.After(delay):
	}
	_ = adapterObj.Call("org.bluez.Adapter1.StopDiscovery", 0).Err
}

func normalizeAddress(in string) string {
	a := strings.TrimSpace(strings.ToUpper(in))
	a = strings.ReplaceAll(a, "-", ":")
	return a
}

func findDevicePath(conn *dbus.Conn, address string) (dbus.ObjectPath, error) {
	managed, err := managedObjects(conn)
	if err != nil {
		return "", err
	}
	for path, ifaces := range managed {
		props, ok := ifaces["org.bluez.Device1"]
		if !ok {
			continue
		}
		v, ok := props["Address"]
		if !ok {
			continue
		}
		addr, ok := v.Value().(string)
		if !ok {
			continue
		}
		if normalizeAddress(addr) == address {
			return path, nil
		}
	}
	return "", fmt.Errorf("device %s not found in BlueZ managed objects", address)
}

func waitCharacteristicPath(ctx context.Context, conn *dbus.Conn, devicePath dbus.ObjectPath, uuid string) (dbus.ObjectPath, error) {
	wantUUID := strings.ToLower(strings.TrimSpace(uuid))
	for {
		path, ok, err := findCharacteristicPath(conn, devicePath, wantUUID)
		if err != nil {
			return "", err
		}
		if ok {
			return path, nil
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("characteristic %s not found: %w", uuid, ctx.Err())
		case <-time.After(lookupInterval):
		}
	}
}

func findCharacteristicPath(conn *dbus.Conn, devicePath dbus.ObjectPath, uuidLower string) (dbus.ObjectPath, bool, error) {
	managed, err := managedObjects(conn)
	if err != nil {
		return "", false, err
	}

	devicePrefix := string(devicePath)
	for path, ifaces := range managed {
		if !strings.HasPrefix(string(path), devicePrefix) {
			continue
		}
		props, ok := ifaces["org.bluez.GattCharacteristic1"]
		if !ok {
			continue
		}
		v, ok := props["UUID"]
		if !ok {
			continue
		}
		u, ok := v.Value().(string)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(u)) == uuidLower {
			return path, true, nil
		}
	}
	return "", false, nil
}

func managedObjects(conn *dbus.Conn) (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	obj := conn.Object(bluezService, "/")
	var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := obj.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&managed); err != nil {
		return nil, fmt.Errorf("get managed objects failed: %w", err)
	}
	return managed, nil
}

func waitDeviceReady(ctx context.Context, conn *dbus.Conn, devicePath dbus.ObjectPath) error {
	connected, resolved, err := readDeviceFlags(conn, devicePath)
	if err != nil {
		return err
	}
	if connected && resolved {
		return nil
	}

	sigCh := make(chan *dbus.Signal, signalBufSize)
	conn.Signal(sigCh)
	defer conn.RemoveSignal(sigCh)

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(devicePath),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return fmt.Errorf("add device properties signal match failed: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("device did not become ready: %w", ctx.Err())
		case sig := <-sigCh:
			if sig == nil || sig.Path != devicePath || len(sig.Body) < 2 {
				continue
			}
			iface, ok := sig.Body[0].(string)
			if !ok || iface != "org.bluez.Device1" {
				continue
			}
			changed, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			if v, ok := changed["Connected"]; ok {
				if b, ok := v.Value().(bool); ok {
					connected = b
				}
			}
			if v, ok := changed["ServicesResolved"]; ok {
				if b, ok := v.Value().(bool); ok {
					resolved = b
				}
			}
			if connected && resolved {
				return nil
			}
		}
	}
}

func readDeviceFlags(conn *dbus.Conn, devicePath dbus.ObjectPath) (connected bool, servicesResolved bool, err error) {
	obj := conn.Object(bluezService, devicePath)
	var cVar dbus.Variant
	if callErr := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.bluez.Device1", "Connected").Store(&cVar); callErr != nil {
		return false, false, fmt.Errorf("read device Connected failed: %w", callErr)
	}
	var sVar dbus.Variant
	if callErr := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.bluez.Device1", "ServicesResolved").Store(&sVar); callErr != nil {
		return false, false, fmt.Errorf("read device ServicesResolved failed: %w", callErr)
	}

	cb, ok := cVar.Value().(bool)
	if !ok {
		return false, false, errors.New("invalid type for Connected property")
	}
	sb, ok := sVar.Value().(bool)
	if !ok {
		return false, false, errors.New("invalid type for ServicesResolved property")
	}
	return cb, sb, nil
}

func isBlueZAlreadyConnected(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "already connected")
}

func shouldRetryConnectError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "le-connection-abort-by-local") ||
		strings.Contains(s, "operation already in progress") ||
		strings.Contains(s, "in progress") ||
		strings.Contains(s, "busy") ||
		strings.Contains(s, "failed")
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isBlueZAlreadyNotifying(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "already notifying")
}

type frskyDecoder struct {
	state int
	buf   []byte
}

func newFrSkyDecoder() *frskyDecoder {
	return &frskyDecoder{buf: make([]byte, 0, 14)}
}

func (d *frskyDecoder) appendTrainerByte(b byte) {
	d.buf = append(d.buf, b)
	if b == 0x0A {
		d.buf = d.buf[:0]
	}
}

func (d *frskyDecoder) feedByte(b byte) ([]int, bool) {
	switch d.state {
	case 1:
		if b == startStop {
			d.state = 2
			d.buf = d.buf[:0]
		} else {
			d.appendTrainerByte(b)
		}
	case 2:
		if b == byteStuff {
			d.state = 3
		} else if b == startStop {
			d.state = 2
			d.buf = d.buf[:0]
		} else {
			d.appendTrainerByte(b)
		}
	case 3:
		d.appendTrainerByte(b ^ stuffMask)
		d.state = 2
	default:
		if b == startStop {
			d.buf = d.buf[:0]
			d.state = 1
		} else {
			d.appendTrainerByte(b)
		}
	}

	if len(d.buf) < 14 {
		return nil, false
	}
	frame := append([]byte(nil), d.buf[:14]...)
	d.buf = d.buf[:0]
	d.state = 0

	var crc byte
	for i := 0; i < 13; i++ {
		crc ^= frame[i]
	}
	if crc != frame[13] || frame[0] != 0x80 {
		return nil, false
	}

	channels := make([]int, 0, btChannels)
	i := 1
	for ch := 0; ch < btChannels; ch += 2 {
		ch1 := int(frame[i]) + (int(frame[i+1]&0xF0) << 4)
		ch2 := (int(frame[i+1]&0x0F) << 4) + (int(frame[i+2]&0xF0) >> 4) + (int(frame[i+2]&0x0F) << 8)
		channels = append(channels, ch1, ch2)
		i += 3
	}
	return channels, true
}
