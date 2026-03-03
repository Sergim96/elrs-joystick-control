// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package devices

import (
	"encoding/json"
	"github.com/kaack/elrs-joystick-control/pkg/util"
	"github.com/veandco/go-sdl2/sdl"
	"sync"
)

type InputGamepad struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	Joy     *sdl.Joystick `json:"-"`
	virtual *virtualState `json:"-"`
}

type virtualState struct {
	mu      sync.RWMutex
	axes    []util.RawValue
	buttons []util.RawValue
	hats    []util.RawValue
}

func (d *InputGamepad) Axis(axis int) util.RawValue {
	if d.virtual != nil {
		d.virtual.mu.RLock()
		defer d.virtual.mu.RUnlock()
		if axis < 0 || axis >= len(d.virtual.axes) {
			return util.ZeroRaw
		}
		return d.virtual.axes[axis]
	}

	return util.RawValue(d.Joy.Axis(axis))
}

func (d *InputGamepad) Button(button int) util.RawValue {
	if d.virtual != nil {
		d.virtual.mu.RLock()
		defer d.virtual.mu.RUnlock()
		if button < 0 || button >= len(d.virtual.buttons) {
			return util.ZeroRaw
		}
		return d.virtual.buttons[button]
	}

	return util.RawValue(d.Joy.Button(button))
}

// Hat returns two axis-like values per physical hat: even indexes are horizontal (left/right),
// odd indexes are vertical (up/down). SDL hats are bitmasks, so we convert the bits into
// the full RawValue range to match axes/buttons semantics.
func (d *InputGamepad) Hat(hat int) util.RawValue {
	if d.virtual != nil {
		d.virtual.mu.RLock()
		defer d.virtual.mu.RUnlock()
		if hat < 0 || hat >= len(d.virtual.hats) {
			return util.ZeroRaw
		}
		return d.virtual.hats[hat]
	}

	physicalHat := hat / 2
	axisComponent := hat % 2

	if physicalHat >= int(d.Joy.NumHats()) {
		return util.ZeroRaw
	}

	state := d.Joy.Hat(physicalHat)

	if axisComponent == 0 {
		if state&sdl.HAT_LEFT != 0 {
			return util.MinRaw
		}
		if state&sdl.HAT_RIGHT != 0 {
			return util.MaxRaw
		}
		return util.ZeroRaw
	}

	if state&sdl.HAT_UP != 0 {
		return util.MinRaw
	}
	if state&sdl.HAT_DOWN != 0 {
		return util.MaxRaw
	}
	return util.ZeroRaw
}

func (d *InputGamepad) Close() {
	if d.Joy != nil {
		d.Joy.Close()
	}
}

func (d *InputGamepad) InstanceId() int32 {
	if d.Joy == nil {
		return 0
	}
	return int32(d.Joy.InstanceID())
}
func (d *InputGamepad) Axes() int32 {
	if d.virtual != nil {
		d.virtual.mu.RLock()
		defer d.virtual.mu.RUnlock()
		return int32(len(d.virtual.axes))
	}
	return int32(d.Joy.NumAxes())
}

func (d *InputGamepad) Buttons() int32 {
	if d.virtual != nil {
		d.virtual.mu.RLock()
		defer d.virtual.mu.RUnlock()
		return int32(len(d.virtual.buttons))
	}
	return int32(d.Joy.NumButtons())
}

func (d *InputGamepad) Hats() int32 {
	if d.virtual != nil {
		d.virtual.mu.RLock()
		defer d.virtual.mu.RUnlock()
		return int32(len(d.virtual.hats))
	}
	return int32(d.Joy.NumHats()) * 2
}

func NewDevice(joy *sdl.Joystick) InputGamepad {
	return InputGamepad{
		Id:   GetJoyStickId(joy),
		Name: joy.Name(),
		Joy:  joy,
	}
}

func NewVirtualDevice(id, name string, axesCount, buttonsCount, hatsCount int) *InputGamepad {
	if axesCount < 0 {
		axesCount = 0
	}
	if buttonsCount < 0 {
		buttonsCount = 0
	}
	if hatsCount < 0 {
		hatsCount = 0
	}

	return &InputGamepad{
		Id:   id,
		Name: name,
		virtual: &virtualState{
			axes:    make([]util.RawValue, axesCount),
			buttons: make([]util.RawValue, buttonsCount),
			hats:    make([]util.RawValue, hatsCount),
		},
	}
}

func (d *InputGamepad) SetVirtualAxes(values []util.RawValue) {
	if d.virtual == nil {
		return
	}
	d.virtual.mu.Lock()
	defer d.virtual.mu.Unlock()
	n := len(d.virtual.axes)
	if len(values) < n {
		n = len(values)
	}
	copy(d.virtual.axes[:n], values[:n])
}

type FakeInputGamepad InputGamepad

func (d *InputGamepad) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		FakeInputGamepad
		Axes    int32 `json:"axes"`
		Buttons int32 `json:"buttons"`
		Hats    int32 `json:"hats"`
	}{
		FakeInputGamepad: FakeInputGamepad(*d),
		Axes:             d.Axes(),
		Buttons:          d.Buttons(),
		Hats:             d.Hats(),
	})
}
