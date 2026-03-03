// SPDX-FileCopyrightText: © 2023 OneEyeFPV oneeyefpv@gmail.com
// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-License-Identifier: FS-0.9-or-later

import React, {useCallback, useEffect, useState} from "react";
import Box from "@mui/material/Box";
import Card from "@mui/material/Card";
import Typography from "@mui/material/Typography";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import CircularProgress from "@mui/material/CircularProgress";
import BluetoothIcon from "@mui/icons-material/Bluetooth";
import {connectBluetoothDevice, getBluetoothDevices, scanBluetoothDevices} from "../misc/server";
import {showError, showSuccess} from "../misc/notifications";

function DeviceCard({device, selected, onConnect}) {
    const isSelected = selected && selected.address === device.address;

    return (
        <Card elevation={4} style={{
            padding: 12,
            marginBottom: 12,
            borderRadius: 12,
            backgroundColor: isSelected ? "#eaf6ff" : "#f8f8f8",
            border: isSelected ? "1px solid #4a90e2" : "1px solid transparent"
        }}>
            <Box style={{display: "flex", justifyContent: "space-between", gap: 12, alignItems: "center"}}>
                <Box>
                    <Typography style={{fontWeight: 700}}>
                        {device.name ? `${device.name} (${device.address})` : device.address}
                    </Typography>
                    {isSelected && <Typography variant="body2" style={{color: "#1e6bb8"}}>Selected</Typography>}
                </Box>
                <Button variant={isSelected ? "outlined" : "contained"} onClick={() => onConnect(device.address)}>
                    {isSelected ? "Selected" : "Connect"}
                </Button>
            </Box>
        </Card>
    );
}

export default function BluetoothDevicesPage() {
    const [state, setState] = useState({devices: [], selected: null});
    const [loading, setLoading] = useState(false);

    const loadState = useCallback(async () => {
        let res = await getBluetoothDevices();
        setState(res || {devices: [], selected: null});
    }, []);

    const handleScan = useCallback(async () => {
        setLoading(true);
        try {
            const res = await scanBluetoothDevices(6);
            setState(res || {devices: [], selected: null});
            showSuccess("Bluetooth scan complete");
        } catch (err) {
            showError(err.message || "Bluetooth scan failed");
        } finally {
            setLoading(false);
        }
    }, []);

    const handleConnect = useCallback(async (address) => {
        try {
            const res = await connectBluetoothDevice(address);
            setState(res || {devices: [], selected: null});
            showSuccess("Bluetooth device selected");
        } catch (err) {
            showError(err.message || "Could not select Bluetooth device");
        }
    }, []);

    useEffect(() => {
        loadState().catch(err => {
            console.error(err);
        });
    }, []);

    const devices = Array.isArray(state?.devices) ? state.devices : [];
    const selected = state?.selected || null;

    return (
        <Box style={{maxWidth: 860, margin: "0 auto", padding: 12}}>
            <Stack direction="row" spacing={2} alignItems="center" style={{marginBottom: 16}}>
                <Button variant="contained" onClick={handleScan} disabled={loading}>
                    {loading ? "Scanning..." : "Scan Bluetooth Devices"}
                </Button>
                {loading && <CircularProgress size={20}/>}
                {selected && (
                    <Typography variant="body2">
                        Selected: <span style={{fontFamily: "monospace"}}>{selected.address}</span>
                        {selected.name ? ` (${selected.name})` : ""}
                    </Typography>
                )}
            </Stack>

            {state?.connectMessage && (
                <Card style={{padding: 10, marginBottom: 12, backgroundColor: "#fff8e1"}}>
                    <Typography variant="body2">{state.connectMessage}</Typography>
                </Card>
            )}

            {state?.lastScanError && (
                <Card style={{padding: 10, marginBottom: 12, backgroundColor: "#fdecea"}}>
                    <Typography variant="body2" style={{color: "#b3261e"}}>
                        Scan error: {state.lastScanError}
                    </Typography>
                </Card>
            )}

            {devices.length === 0 ? (
                <Card style={{padding: 20, borderRadius: 12}}>
                    <Typography align="center">
                        No Bluetooth devices loaded yet. Click "Scan Bluetooth Devices".
                    </Typography>
                </Card>
            ) : (
                devices.map((device) => (
                    <DeviceCard
                        key={device.address}
                        device={device}
                        selected={selected}
                        onConnect={handleConnect}
                    />
                ))
            )}
        </Box>
    );
}

BluetoothDevicesPage.id = "bluetooth-devices";
BluetoothDevicesPage.title = "Bluetooth Devices";
BluetoothDevicesPage.menuIcon = <BluetoothIcon/>;
