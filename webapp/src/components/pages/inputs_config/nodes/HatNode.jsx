// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

import React from "react";
import {SvgIcon} from "@mui/material";
import {TopOutput} from "../handles/TopOutput";
import {BottomInput} from "../handles/BottomInput";

import {GenericInputNode} from "./GenericInputNode";

function HatNode(node) {
    return (<GenericInputNode
        node={node}
        valueProps={{
            style: {marginTop: "-2px", marginBottom: "1px"}
        }}
        iconProps={{
            style: {}
        }}
        labelProps={{
            style: {marginTop: "2px", marginBottom: "-2px"},
            appendField: "number"
        }}
    >

        <TopOutput node={node}/>

        <BottomInput node={node} fieldName={"input"}/>
    </GenericInputNode>);
}

HatNode.type = "hat";
HatNode.menuIcon = (
  <SvgIcon viewBox="0 0 24 24">
    <path
      fill="#656565"
      d="m 10.666398,2.7947062 h 2 v 6 h 6 v 1.9999998 h -6 v 6 h -2 v -6 H 4.6663975 V 8.7947062 h 6.0000005 z m -2.0000005,7.9999998 2.0016155,-0.02728 -0.0016,2.027277 v -2 z m 6.0000005,0 v -0.0016 h 2 v 0.0016 z"
    />
    <rect
      fill="#656565"
      width="8.2960491"
      height="1.8114309"
      x="7.518373"
      y="2.7947061"
    />
    <rect
      fill="#656565"
      width="8.2960491"
      height="1.8114309"
      x="7.518373"
      y="14.983275"
    />
    <rect
      fill="#656565"
      width="8.2960491"
      height="1.8114309"
      x="-13.942731"
      y="4.6663976"
      transform="rotate(-90)"
    />
    <rect
      fill="#656565"
      width="8.2960491"
      height="1.8114309"
      x="-13.942731"
      y="16.854967"
      transform="rotate(-90)"
    />
  </SvgIcon>
);

export default HatNode;
