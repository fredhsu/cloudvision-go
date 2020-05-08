// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package devices

import (
	"fmt"

	"github.com/aristanetworks/cloudvision-go/device"
	"github.com/aristanetworks/cloudvision-go/provider"
	psonic "github.com/aristanetworks/cloudvision-go/provider/sonic"
)

// Register this device with its options.
func init() {
	options := map[string]device.Option{
		"pollInterval": {
			Description: "Polling interval, with unit suffix (s/m/h)",
			Default:     "20s",
		},
	}
	device.Register("sonic", NewSonicDevice, options)
}

type sonic struct {
	deviceID string
	provider provider.GNMIProvider
}

func (d *sonic) Alive() (bool, error) {
	// Runs on the device itself, so if the method is called, it's alive.
	return true, nil
}

// Use the device's serial number as its ID.
func (d *sonic) deviceSerial() (string, error) {
	return "JPE16194299", nil
}

func (d *sonic) DeviceID() (string, error) {
	return d.deviceID, nil
}

func (d *sonic) Providers() ([]provider.Provider, error) {
	return []provider.Provider{d.provider}, nil
}

// NewSonicDevice instantiates a Sonic device.
func NewSonicDevice(options map[string]string) (device.Device, error) {
	pollInterval, err := device.GetDurationOption("pollInterval", options)
	if err != nil {
		return nil, err
	}

	device := sonic{}
	did, err := device.deviceSerial()
	if err != nil {
		return nil, fmt.Errorf("Failure getting device ID: %v", err)
	}
	device.deviceID = did
	device.provider = psonic.NewSonicProvider(pollInterval)

	return &device, nil
}
