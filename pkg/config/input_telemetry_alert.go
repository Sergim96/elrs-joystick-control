// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package config

import (
	"strings"

	"github.com/kaack/elrs-joystick-control/pkg/util"
)

const (
	TelemetryComparatorAbove = "above"
	TelemetryComparatorBelow = "below"
)

type TelemetryAlertT struct {
	Source     string  `json:"source"`
	Comparator string  `json:"comparator"`
	Threshold  float32 `json:"threshold"`
	Track      string  `json:"track"`
	CooldownMS *int32  `json:"cooldown_ms"`
}

// InputTelemetryAlert represents a telemetry threshold -> audio trigger node.
type InputTelemetryAlert struct {
	Id    string        `json:"id"`
	Value util.RawValue `json:"value"`
	IsNaN bool          `json:"-"`

	Type      string          `json:"type"`
	Telemetry TelemetryAlertT `json:"telemetry_alert" input:"true"`
}

func (i *InputTelemetryAlert) Eval(c *Config) (src IOType, out util.RawValue, ch util.ChannelNumber, nan bool) {
	return nil, 0, -1, true
}

func (i *InputTelemetryAlert) InputType() string {
	return i.Type
}

func (i *InputTelemetryAlert) InputValue() *util.RawValue {
	return nil
}

func (i *InputTelemetryAlert) InputId() string {
	return i.Id
}

func (i *InputTelemetryAlert) Children() (out *[]*IOHolder) {
	return nil
}

func (t *TelemetryAlertT) TrackName() string {
	return strings.TrimSpace(t.Track)
}
