// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

import React from "react";
import {GenericInputNode} from "./GenericInputNode";
import {BottomInput} from "../handles/BottomInput";
import {SvgIcon} from "@mui/material";

function AudioNode(node) {
    return (<GenericInputNode
        node={node}
        valueProps={{
            style: {marginTop: "-1px", marginBottom: "2px"}
        }}
        iconProps={{
            style: {}
        }}
        labelProps={{
            style: {marginTop: "1px", marginBottom: "-2px"},
            appendField: "track"
        }}
    >
        <BottomInput node={node} fieldName={"input"}/>
    </GenericInputNode>);
}

AudioNode.type = "audio";
AudioNode.menuIcon = <SvgIcon>
    <path fill="#656565"
          d="M13 4.05v11.22A4.5 4.5 0 1 1 11 11V6H7V4h6m2-2h4v2h-2v12.14c0 2.32-1.88 4.4-4.34 4.81A4.505 4.505 0 0 1 8 16.5c0-1.61.83-3.02 2.07-3.84c.6.75 1.52 1.25 2.60 1.34c.13.01.26.02.39.02c.66 0 1.3-.17 1.87-.47V2Z"/>
</SvgIcon>;

export default AudioNode;
