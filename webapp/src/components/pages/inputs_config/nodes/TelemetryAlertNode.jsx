// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

import React from "react";
import {SvgIcon} from "@mui/material";
import {GenericInputNode} from "./GenericInputNode";

function TelemetryAlertNode(node) {
    return (<GenericInputNode
        node={node}
        iconProps={{
            style: {}
        }}
        labelProps={{
            style: {marginTop: "2px", marginBottom: "-2px"}
        }}
    >
    </GenericInputNode>);
}

TelemetryAlertNode.type = "telemetry_alert";
TelemetryAlertNode.menuIcon = <SvgIcon>
    <path fill="#656565"
          d="M12 5L2 19h20L12 5Zm0 3.84L18.46 17H5.54L12 8.84Zm-1 2.66v3.5h2v-3.5h-2Zm0 4.5v2h2v-2h-2Z"/>
</SvgIcon>;

export default TelemetryAlertNode;
