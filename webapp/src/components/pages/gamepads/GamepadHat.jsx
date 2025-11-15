// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

import React from "react";
import {Progress, ProgressSkeleton} from "../misc/Progress";

export const hatValueToPercent = function (value) {
    let clamped = value;
    if (clamped > 1) {
        clamped = 1;
    } else if (clamped < -1) {
        clamped = -1;
    }

    return (clamped + 1) / 2 * 100;
};

export const GamepadHat = function ({index, value, loading}) {
    return <Progress index={index} value={value} loading={loading} scale={hatValueToPercent}/>;
};

export const GamepadHatSkeleton = function () {
    return <ProgressSkeleton loading={true}/>;
};
