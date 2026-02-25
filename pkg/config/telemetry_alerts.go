// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

package config

import (
	"fmt"
	ac "github.com/kaack/elrs-joystick-control/pkg/audio"
	"github.com/kaack/elrs-joystick-control/pkg/proto/generated/pb"
	"strings"
	"time"
)

type telemetryAlertRuntime struct {
	alert       *InputTelemetryAlert
	active      bool
	lastTrigger time.Time
	cooldown    time.Duration
}

func newTelemetryAlertRuntime(alert *InputTelemetryAlert) *telemetryAlertRuntime {
	cooldown := time.Duration(0)
	if alert != nil && alert.Telemetry.CooldownMS != nil && *alert.Telemetry.CooldownMS > 0 {
		cooldown = time.Duration(*alert.Telemetry.CooldownMS) * time.Millisecond
	}
	return &telemetryAlertRuntime{
		alert:    alert,
		cooldown: cooldown,
	}
}

func (r *telemetryAlertRuntime) evaluate(value float64, now time.Time, audioCtl *ac.Controller) {
	if r == nil || r.alert == nil {
		return
	}

	alert := r.alert.Telemetry
	var shouldBeActive bool

	switch alert.Comparator {
	case TelemetryComparatorAbove:
		shouldBeActive = value > float64(alert.Threshold)
	case TelemetryComparatorBelow:
		shouldBeActive = value < float64(alert.Threshold)
	default:
		shouldBeActive = false
	}

	if shouldBeActive {
		if !r.active || (r.cooldown > 0 && now.Sub(r.lastTrigger) >= r.cooldown) {
			track := strings.TrimSpace(alert.Track)
			if track != "" && audioCtl != nil {
				if err := audioCtl.Play(track); err != nil {
					fmt.Printf("telemetry alert: could not play %s: %v\n", track, err)
				} else {
					fmt.Printf("telemetry alert [%s]: value=%.2f triggered track %q\n", r.alert.Id, value, track)
				}
			}
			r.lastTrigger = now
			r.active = true
		}
	} else {
		r.active = false
	}
}

func telemetryValueFromProto(source string, telemetry *pb.Telemetry) (float64, bool) {
	if telemetry == nil {
		return 0, false
	}

	switch source {
	case "battery_voltage":
		if data := telemetry.GetBattery(); data != nil {
			return float64(data.GetVoltage()), true
		}
	case "battery_current":
		if data := telemetry.GetBattery(); data != nil {
			return float64(data.GetCurrent()), true
		}
	case "battery_fuel":
		if data := telemetry.GetBattery(); data != nil {
			return float64(data.GetFuel()), true
		}
	case "battery_remaining":
		if data := telemetry.GetBattery(); data != nil {
			return float64(data.GetRemaining()), true
		}
	case "attitude_pitch":
		if data := telemetry.GetAttitude(); data != nil {
			return float64(data.GetPitch()), true
		}
	case "attitude_roll":
		if data := telemetry.GetAttitude(); data != nil {
			return float64(data.GetRoll()), true
		}
	case "attitude_yaw":
		if data := telemetry.GetAttitude(); data != nil {
			return float64(data.GetYaw()), true
		}
	case "gps_ground_speed":
		if data := telemetry.GetGps(); data != nil {
			return float64(data.GetGroundSpeed()), true
		}
	case "gps_heading":
		if data := telemetry.GetGps(); data != nil {
			return float64(data.GetHeading()), true
		}
	case "gps_altitude":
		if data := telemetry.GetGps(); data != nil {
			return float64(data.GetAltitude()), true
		}
	case "gps_satellites":
		if data := telemetry.GetGps(); data != nil {
			return float64(data.GetSatellites()), true
		}
	case "link_stats_uplink_rssi1":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetUplinkRssi1()), true
		}
	case "link_stats_uplink_rssi2":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetUplinkRssi2()), true
		}
	case "link_stats_uplink_link_quality":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetUplinkLinkQuality()), true
		}
	case "link_stats_uplink_snr":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetUplinkSnr()), true
		}
	case "link_stats_uplink_power":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetUplinkPower()), true
		}
	case "link_stats_downlink_rssi":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetDownlinkRssi()), true
		}
	case "link_stats_downlink_link_quality":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetDownlinkLinkQuality()), true
		}
	case "link_stats_downlink_snr":
		if data := telemetry.GetLinkStats(); data != nil {
			return float64(data.GetDownlinkSnr()), true
		}
	case "link_tx_downlink_rssi":
		if data := telemetry.GetLinkTx(); data != nil {
			return float64(data.GetDownlinkRssi()), true
		}
	case "link_tx_uplink_power":
		if data := telemetry.GetLinkTx(); data != nil {
			return float64(data.GetUplinkPower()), true
		}
	case "link_tx_uplink_fps":
		if data := telemetry.GetLinkTx(); data != nil {
			return float64(data.GetUplinkFps()), true
		}
	case "link_rx_uplink_rssi":
		if data := telemetry.GetLinkRx(); data != nil {
			return float64(data.GetUplinkRssi()), true
		}
	case "link_rx_downlink_power":
		if data := telemetry.GetLinkRx(); data != nil {
			return float64(data.GetDownlinkPower()), true
		}
	case "barometer_altitude":
		if data := telemetry.GetBarometer(); data != nil {
			return float64(data.GetAltitude()), true
		}
	case "variometer_speed":
		if data := telemetry.GetVariometer(); data != nil {
			return float64(data.GetVerticalSpeed()), true
		}
	case "barometer_variometer_altitude":
		if data := telemetry.GetBarometerVariometer(); data != nil {
			return float64(data.GetAltitude()), true
		}
	case "barometer_variometer_vertical_speed":
		if data := telemetry.GetBarometerVariometer(); data != nil {
			return float64(data.GetVerticalSpeed()), true
		}
	case "sync_rate":
		if data := telemetry.GetSync(); data != nil {
			return float64(data.GetRate()), true
		}
	case "sync_offset":
		if data := telemetry.GetSync(); data != nil {
			return float64(data.GetOffset()), true
		}
	}

	return 0, false
}
