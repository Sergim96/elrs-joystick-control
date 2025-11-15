// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package devices

import (
	"fmt"
	"github.com/dlsniper/debugger"
	"github.com/kaack/elrs-joystick-control/pkg/proto/generated/pb"
	"github.com/kaack/elrs-joystick-control/pkg/util"
	"github.com/veandco/go-sdl2/sdl"
	"gopkg.in/tomb.v2"
)

type Controller struct {
	Gamepads         map[string]*InputGamepad
	t                *tomb.Tomb
	DeviceEventCount int32
	DeviceEventChan  chan int32
}

func NewCtl() *Controller {
	devicesCtl := &Controller{}
	err := devicesCtl.Init()

	if err != nil {
		devicesCtl.Quit()
		panic(err)
	}

	return devicesCtl
}

func (c *Controller) Gamepad(id string) (*InputGamepad, bool) {
	res, ok := c.Gamepads[id]
	return res, ok
}

func (c *Controller) Init() (err error) {
	if err = sdl.Init(sdl.INIT_GAMECONTROLLER); err != nil {
		return err
	}
	c.Gamepads = EnumerateDevices()
	c.StartPolling()
	return err
}

func (c *Controller) Quit() {
	if err := c.StopPolling(); err != nil {
		fmt.Printf("error stopping polling loop. %s", err.Error())
	}

	for _, device := range c.Gamepads {
		device.Close()
	}
	sdl.Quit()
}

func (c *Controller) GetGamepadStates(device *InputGamepad, states *pb.GamepadInputsStates) *pb.GamepadInputsStates {

	const (
		hatRawMin    util.RawValue = -32767
		hatRawMax    util.RawValue = 32767
		hatOutputMin util.RawValue = -1
		hatOutputMax util.RawValue = 1
	)

	axisNumber := 0
	axesCount := int(device.Axes())
	buttonNumber := 0
	buttonsCount := int(device.Buttons())
	hatNumber := 0
	hatsCount := int(device.Hats())
	totalInputs := axesCount + hatsCount + buttonsCount

	if states != nil && len(states.InputsStates) != totalInputs {
		states = nil
	}

	if states != nil {
		buttonOffset := axesCount + hatsCount
		for axisNumber = 0; axisNumber < axesCount; axisNumber++ {
			value := int32(device.Axis(axisNumber))
			states.InputsStates[axisNumber].Value = value
			fmt.Printf("controller axis %d value %d\n", axisNumber, value)
		}
		for hatNumber = 0; hatNumber < hatsCount; hatNumber++ {
			value := int32(util.MapRange(device.Hat(hatNumber), hatRawMin, hatRawMax, hatOutputMin, hatOutputMax))
			states.InputsStates[axesCount+hatNumber].Value = value
			fmt.Printf("controller hat %d value %d\n", hatNumber, value)
		}
		for buttonNumber = 0; buttonNumber < buttonsCount; buttonNumber++ {
			value := int32(device.Button(buttonNumber))
			states.InputsStates[buttonOffset+buttonNumber].Value = value
			fmt.Printf("controller button %d value %d\n", buttonNumber, value)
		}
		return states
	}

	inputStates := make([]*pb.GamepadInputState, totalInputs)

	for axisNumber = 0; axisNumber < axesCount; axisNumber++ {
		value := int32(device.Axis(axisNumber))
		inputStates[axisNumber] = &pb.GamepadInputState{
			Type:  pb.GamepadInputType_AXIS,
			Index: int32(axisNumber),
			Value: value,
		}
		fmt.Printf("controller axis %d value %d\n", axisNumber, value)
	}

	for hatNumber = 0; hatNumber < hatsCount; hatNumber++ {
		value := int32(util.MapRange(device.Hat(hatNumber), hatRawMin, hatRawMax, hatOutputMin, hatOutputMax))
		inputStates[axesCount+hatNumber] = &pb.GamepadInputState{
			Type:  pb.GamepadInputType_HAT,
			Index: int32(hatNumber),
			Value: value,
		}
		fmt.Printf("controller hat %d value %d\n", hatNumber, value)
	}

	for buttonNumber = 0; buttonNumber < buttonsCount; buttonNumber++ {
		value := int32(device.Button(buttonNumber))
		inputStates[axesCount+hatsCount+buttonNumber] = &pb.GamepadInputState{
			Type:  pb.GamepadInputType_BUTTON,
			Index: int32(buttonNumber),
			Value: value,
		}
		fmt.Printf("controller button %d value %d\n", buttonNumber, value)
	}

	states = &pb.GamepadInputsStates{InputsStates: inputStates}

	return states
}

func (c *Controller) initDeviceChan() {
	c.DeviceEventCount = 0
	c.DeviceEventChan = make(chan int32)
}

func (c *Controller) AlertDeviceChan() {
	c.DeviceEventCount += 1 //it's okay if it overflows
	select {
	case c.DeviceEventChan <- c.DeviceEventCount:
		//fmt.Printf("event %d sent", c.EventCount)
	default:
		//no-op
	}
}

func (c *Controller) StartPolling() {

	sdl.JoystickEventState(sdl.ENABLE)

	c.t = &tomb.Tomb{}
	c.initDeviceChan()
	c.t.Go(func() error {
		debugger.SetLabels(func() []string {
			return []string{
				"poller",
			}
		})

		//var event sdl.Event
		for {
			select {
			case <-c.t.Dying():
				fmt.Println("(devices): exiting polling loop")
				return nil
			default:
				if event := sdl.PollEvent(); event != nil {
					c.AlertDeviceChan()
				}
			}
		}
	})
}

func (c *Controller) StopPolling() error {
	sdl.JoystickEventState(sdl.DISABLE)
	if c.t == nil {
		return nil
	}

	c.t.Kill(nil)
	if err := c.t.Wait(); err != nil {
		return err
	}
	return nil
}
