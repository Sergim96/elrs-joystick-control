// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kaack/elrs-joystick-control/pkg/util"
)

type AudioT struct {
	Input *IOHolder `json:"input"`
	Track string    `json:"track"`
	Min   *int32    `json:"min_value"`
	Max   *int32    `json:"max_value"`
}

type InputAudio struct {
	Id    string        `json:"id"`
	Value util.RawValue `json:"value"`
	IsNaN bool          `json:"-"`

	Type  string `json:"type"`
	Audio AudioT `json:"audio" input:"true"`

	lastActive bool
}

type fakeAudioT AudioT

func (a *AudioT) UnmarshalJSON(data []byte) error {
	tmp := fakeAudioT{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*a = AudioT(tmp)

	a.Track = strings.TrimSpace(a.Track)
	return nil
}

func (i *InputAudio) Eval(c *Config) (src IOType, out util.RawValue, ch util.ChannelNumber, nan bool) {
	src, out, ch, nan = i.eval(c)
	i.Value = out
	i.IsNaN = nan
	return src, out, ch, nan
}

func (i *InputAudio) eval(c *Config) (src IOType, out util.RawValue, ch util.ChannelNumber, nan bool) {
	input := i.Audio.Input
	if input == nil {
		fmt.Printf("audio node [%s]: no input connected\n", i.Id)
		i.lastActive = false
		return nil, 0, -1, true
	}

	src, out, ch, nan = input.Eval(c)
	if nan {
		fmt.Printf("audio node [%s]: input not available (NaN)\n", i.Id)
		i.lastActive = false
		return src, out, ch, nan
	}

	fmt.Printf("audio node [%s]: input value=%d\n", i.Id, out)

	isActive := i.isActiveValue(out)
	if isActive && !i.lastActive {
		track := strings.TrimSpace(i.Audio.Track)
		if track != "" && c != nil && c.Ctl != nil && c.Ctl.audioCtl != nil {
			fmt.Printf("audio node [%s]: trigger rising edge (value=%d) playing track %q\n", i.Id, out, track)
			if err := c.Ctl.audioCtl.Play(track); err != nil {
				fmt.Printf("audio node: could not play %s: %v\n", track, err)
			}
		} else if track == "" {
			fmt.Printf("audio node [%s]: trigger high but no track selected\n", i.Id)
		} else {
			fmt.Printf("audio node [%s]: trigger high but audio controller not ready\n", i.Id)
		}
	} else if !isActive && i.lastActive {
		fmt.Printf("audio node [%s]: trigger falling edge (value=%d)\n", i.Id, out)
	}
	i.lastActive = isActive

	return src, out, ch, false
}

func (i *InputAudio) InputType() string {
	return i.Type
}

func (i *InputAudio) InputValue() *util.RawValue {
	if i.IsNaN {
		return nil
	}
	return &i.Value
}

func (i *InputAudio) InputId() string {
	return i.Id
}

func (i *InputAudio) Children() (out *[]*IOHolder) {
	return GetChildren(i.Audio.Input, nil)
}

func (i *InputAudio) isActiveValue(value util.RawValue) bool {
	if i.Audio.Min == nil && i.Audio.Max == nil {
		return value != util.ZeroRaw
	}

	minVal := util.MinRaw
	maxVal := util.MaxRaw

	if i.Audio.Min != nil {
		minVal = util.RawValue(*i.Audio.Min)
	}

	if i.Audio.Max != nil {
		maxVal = util.RawValue(*i.Audio.Max)
	}

	if minVal > maxVal {
		minVal, maxVal = maxVal, minVal
	}

	return value >= minVal && value <= maxVal
}
