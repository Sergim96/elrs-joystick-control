// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package main

import (
	"context"
	"flag"
	"fmt"
	ac "github.com/kaack/elrs-joystick-control/pkg/audio"
	"github.com/kaack/elrs-joystick-control/pkg/client"
	cc "github.com/kaack/elrs-joystick-control/pkg/config"
	dc "github.com/kaack/elrs-joystick-control/pkg/devices"
	bt "github.com/kaack/elrs-joystick-control/pkg/headtrackerbt"
	hc "github.com/kaack/elrs-joystick-control/pkg/http"
	lc "github.com/kaack/elrs-joystick-control/pkg/link"
	sc "github.com/kaack/elrs-joystick-control/pkg/serial"
	gc "github.com/kaack/elrs-joystick-control/pkg/server"
	"github.com/kaack/elrs-joystick-control/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"
)

const headTrackerGamepadID = "headtracker-bt"
const headTrackerChannels = 8

func mapHeadTrackerChannelToRaw(v int) util.RawValue {
	const center = 1500
	const span = 512

	delta := v - center
	if delta > span {
		delta = span
	} else if delta < -span {
		delta = -span
	}
	raw := (delta * int(util.MaxRaw)) / span
	if raw > int(util.MaxRaw) {
		raw = int(util.MaxRaw)
	} else if raw < int(util.MinRaw) {
		raw = int(util.MinRaw)
	}
	return util.RawValue(raw)
}

func main() {

	webAppPort := new(int)
	flag.IntVar(webAppPort, "webapp-port", 3000, "Web Application port number")

	grpcPort := new(int)
	flag.IntVar(grpcPort, "grpc-port", 10000, "gRPC Server port number")

	txServerPortName := new(string)
	flag.StringVar(txServerPortName, "tx-serial-port-name", "", "tx Serial port name")

	txServerPortBaudRate := new(int)
	flag.IntVar(txServerPortBaudRate, "tx-serial-port-baud-rate", 921600, "tx Serial port baud rate")

	configFilePath := new(string)
	flag.StringVar(configFilePath, "config-file-path", "", "config json file path")

	audioDir := new(string)
	flag.StringVar(audioDir, "audio-dir", "./audio", "Directory that holds audio prompts (mp3 files)")

	disableWebUI := new(bool)
	flag.BoolVar(disableWebUI, "disable-web-ui", false, "disable the Web-UI HTTP server")

	headTrackerBTScan := new(bool)
	flag.BoolVar(headTrackerBTScan, "headtracker-bt-scan", false, "scan BLE devices and prompt to choose a HeadTracker before startup")

	headTrackerBTScanSeconds := new(int)
	flag.IntVar(headTrackerBTScanSeconds, "headtracker-bt-scan-seconds", 6, "seconds to scan for BLE devices when -headtracker-bt-scan is enabled")

	headTrackerBTAddress := new(string)
	flag.StringVar(headTrackerBTAddress, "headtracker-bt-address", "", "HeadTracker BLE MAC address (selected interactively when -headtracker-bt-scan is used)")

	headTrackerBTConfigPath := new(string)
	flag.StringVar(headTrackerBTConfigPath, "headtracker-bt-config-path", "./headtracker-bt.json", "Path to persisted HeadTracker Bluetooth device selection")

	headTrackerBTAutoTimeout := new(int)
	flag.IntVar(headTrackerBTAutoTimeout, "headtracker-bt-autoconnect-timeout-seconds", 10, "Auto-connect scan timeout in seconds for persisted HeadTracker Bluetooth selection")

	flag.Parse()
	bt.DefaultManager.SetConfigPath(*headTrackerBTConfigPath)

	if *headTrackerBTAutoTimeout <= 0 {
		*headTrackerBTAutoTimeout = 10
	}

	autoCtx, autoCancel := context.WithTimeout(context.Background(), time.Duration(*headTrackerBTAutoTimeout+5)*time.Second)
	autoState := bt.DefaultManager.AutoConnectFromConfig(autoCtx, time.Duration(*headTrackerBTAutoTimeout)*time.Second)
	autoCancel()
	if autoState.LastScanError != "" {
		fmt.Printf("HeadTracker Bluetooth auto-connect error: %s\n", autoState.LastScanError)
	} else if autoState.Selected != nil {
		fmt.Printf("HeadTracker Bluetooth auto-selected: %s (%s)\n", autoState.Selected.Address, autoState.Selected.Name)
	}

	if *headTrackerBTScan {
		scanCtx, cancel := context.WithTimeout(context.Background(), time.Duration(*headTrackerBTScanSeconds+5)*time.Second)
		defer cancel()

		fmt.Printf("Scanning Bluetooth devices for %d seconds...\n", *headTrackerBTScanSeconds)
		scanState := bt.DefaultManager.Scan(scanCtx, time.Duration(*headTrackerBTScanSeconds)*time.Second)
		if scanState.LastScanError != "" {
			fmt.Printf("Bluetooth scan failed: %s\n", scanState.LastScanError)
		} else {
			selected, err := bt.PromptSelectDevice(os.Stdin, os.Stdout, scanState.Devices)
			if err != nil {
				fmt.Printf("Bluetooth device selection failed: %s\n", err.Error())
			} else {
				state, ok := bt.DefaultManager.Select(selected.Address)
				if !ok {
					fmt.Printf("Bluetooth device selection failed: device %s is no longer available\n", selected.Address)
				} else {
					*headTrackerBTAddress = selected.Address
					fmt.Printf("Selected Bluetooth device: %s (%s)\n", selected.Address, selected.Name)
					if state.ConnectMessage != "" {
						fmt.Println(state.ConnectMessage)
					}
				}
			}
		}
	}

	if *headTrackerBTAddress != "" {
		fmt.Printf("HeadTracker Bluetooth target configured: %s\n", *headTrackerBTAddress)
	}

	grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
	reflection.Register(grpcServer)

	audioCtl := ac.NewCtl(*audioDir)
	defer audioCtl.Quit()

	devicesCtl := dc.NewCtl()
	defer devicesCtl.Quit()

	btMonitorCtx, btMonitorCancel := context.WithCancel(context.Background())
	defer btMonitorCancel()
	go func() {
		var activeAddress string
		var activeStreamCancel context.CancelFunc

		startStream := func(selected *bt.Device) {
			if selected == nil {
				return
			}
			deviceName := selected.Name
			if deviceName == "" {
				deviceName = selected.Address
			}
			virtualGamepad := dc.NewVirtualDevice(headTrackerGamepadID, fmt.Sprintf("HeadTracker BLE (%s)", deviceName), headTrackerChannels, 0, 0)
			devicesCtl.UpsertGamepad(virtualGamepad)

			streamCtx, cancel := context.WithCancel(btMonitorCtx)
			activeStreamCancel = cancel
			go func(address string, dev *dc.InputGamepad) {
				err := bt.StreamChannels(streamCtx, address, func(channels []int) {
					axes := make([]util.RawValue, headTrackerChannels)
					for i := 0; i < headTrackerChannels; i++ {
						if i < len(channels) {
							axes[i] = mapHeadTrackerChannelToRaw(channels[i])
						} else {
							axes[i] = util.ZeroRaw
						}
					}
					dev.SetVirtualAxes(axes)
					devicesCtl.AlertDeviceChan()
				})
				if err != nil {
					fmt.Printf("HeadTracker Bluetooth stream error: %s\n", err.Error())
				}
			}(selected.Address, virtualGamepad)
		}

		applySelection := func(selected *bt.Device) {
			addr := ""
			if selected != nil {
				addr = selected.Address
			}
			if addr == activeAddress {
				return
			}

			if activeStreamCancel != nil {
				activeStreamCancel()
				activeStreamCancel = nil
			}
			activeAddress = addr
			if selected == nil {
				return
			}
			startStream(selected)
		}

		applySelection(bt.DefaultManager.Selected())

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-btMonitorCtx.Done():
				if activeStreamCancel != nil {
					activeStreamCancel()
				}
				return
			case <-ticker.C:
				applySelection(bt.DefaultManager.Selected())
			}
		}
	}()

	configCtl := cc.NewCtl(devicesCtl, audioCtl)
	defer configCtl.Quit()

	httpCtl := hc.NewCtl(*webAppPort, grpcServer, audioCtl)
	defer httpCtl.Quit()

	serialCtl := sc.NewCtl()
	defer serialCtl.Quit()

	linkCtl := lc.NewCtl(devicesCtl, serialCtl, configCtl)
	defer linkCtl.Quit()

	serverCtl := gc.NewCtl(*grpcPort, grpcServer, devicesCtl, serialCtl, configCtl, linkCtl, httpCtl)
	defer serverCtl.Quit()

	// Automatically configure through gprc when conditions are met
	client.Init(*txServerPortName, *configFilePath, *txServerPortBaudRate, *grpcPort, *disableWebUI)

	go func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, os.Interrupt)
		<-sigChan
		fmt.Println("Ctrl-C detected, exiting")
		if err := serverCtl.Stop(); err != nil {
			fmt.Printf("could not stop server controller. %s\n", err.Error())
		}
	}()

	serverCtl.Wait()
}
