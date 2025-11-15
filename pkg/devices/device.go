// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package devices

import (
	"encoding/json"
	"github.com/kaack/elrs-joystick-control/pkg/util"
	"github.com/veandco/go-sdl2/sdl"
)

type InputGamepad struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	Joy *sdl.Joystick `json:"-"`
}

func (d *InputGamepad) Axis(axis int) util.RawValue {
	return util.RawValue(d.Joy.Axis(axis))
}

func (d *InputGamepad) Button(button int) util.RawValue {
	return util.RawValue(d.Joy.Button(button))
}

// Hat returns two axis-like values per physical hat: even indexes are horizontal (left/right),
// odd indexes are vertical (up/down). SDL hats are bitmasks, so we convert the bits into
// the full RawValue range to match axes/buttons semantics.
func (d *InputGamepad) Hat(hat int) util.RawValue {
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
	d.Joy.Close()
}

func (d *InputGamepad) InstanceId() int32 {
	return int32(d.Joy.InstanceID())
}
func (d *InputGamepad) Axes() int32 {
	return int32(d.Joy.NumAxes())
}

func (d *InputGamepad) Buttons() int32 {
	return int32(d.Joy.NumButtons())
}

func (d *InputGamepad) Hats() int32 {
	return int32(d.Joy.NumHats()) * 2
}

func NewDevice(joy *sdl.Joystick) InputGamepad {
	return InputGamepad{
		Id:   GetJoyStickId(joy),
		Name: joy.Name(),
		Joy:  joy,
	}
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
