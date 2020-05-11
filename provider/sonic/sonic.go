// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package sonic

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aristanetworks/cloudvision-go/provider"
	pgnmi "github.com/aristanetworks/cloudvision-go/provider/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi"
)

type sonic struct {
	client       gnmi.GNMIClient
	errc         chan error
	pollInterval time.Duration
}

// Return a set of gNMI updates for in/out bytes, packets, and errors
// for a given interface, as reported by netstat.
func updatesFromSystem() ([]*gnmi.Update, error) {

	return []*gnmi.Update{
		// Platform components to define teh part number and serial number
		pgnmi.Update(pgnmi.PlatformComponentPath("SONiC", "name"),
			pgnmi.Strval("SONiC")),
		pgnmi.Update(pgnmi.PlatformComponentConfigPath("SONiC", "name"),
			pgnmi.Strval("SONiC")),
		pgnmi.Update(pgnmi.PlatformComponentStatePath("SONiC", "name"),
			pgnmi.Strval("SONiC")),
		pgnmi.Update(pgnmi.PlatformComponentStatePath("SONiC", "type"),
			pgnmi.Strval("openconfig-platform-types:CHASSIS")),
		pgnmi.Update(pgnmi.PlatformComponentStatePath("SONiC", "software-version"),
			pgnmi.Strval("SONiC.staphylo.108945.1")),
		pgnmi.Update(pgnmi.PlatformComponentStatePath("SONiC", "part-no"),
			pgnmi.Strval("Arista-7050-QX-32S")),
		pgnmi.Update(pgnmi.PlatformComponentStatePath("SONiC", "hardware-version"),
			pgnmi.Strval("Arista-7050-QX-32S")),
		pgnmi.Update(pgnmi.PlatformComponentStatePath("SONiC", "mfg-name"),
			pgnmi.Strval("Arista Networks")),
		pgnmi.Update(SystemConfigPath("hostname"), pgnmi.Strval("sonic-leaf1")),
		pgnmi.Update(SystemStatePath("hostname"), pgnmi.Strval("sonic-leaf1")),
		// TODO: boot time -> uptime and mgmt IP address
		// pgnmi.Update(SystemStatePath("boot-time"), &gnmi.TypedValue{}),
		pgnmi.Update(pgnmi.LldpStatePath("chassis-id"), pgnmi.Strval("00:1c:73:e1:be:ef")),
	}, nil
	// return Path("lldp", "interfaces", ListWithKey("interface", "name",
	// intfName), "state", "counters", leafName)
}

// SystemConfigPath provides an easy gnmi path to the system config settings
func SystemConfigPath(leafName string) *gnmi.Path {
	return pgnmi.Path("system", "config", leafName)
}

// SystemStatePath provides an easy gnmi path to the system state settings
func SystemStatePath(leafName string) *gnmi.Path {
	return pgnmi.Path("system", "state", leafName)
}

func (d *sonic) updatePlatform() ([]*gnmi.SetRequest, error) {

	log.Printf("updatePlatform")
	setRequest := new(gnmi.SetRequest)
	updates, err := updatesFromSystem()
	if err != nil {
		return nil, err
	}

	setRequest.Delete = []*gnmi.Path{pgnmi.Path("components")}
	setRequest.Delete = []*gnmi.Path{pgnmi.Path("system")}
	setRequest.Delete = []*gnmi.Path{pgnmi.Path("lldp")}
	setRequest.Replace = updates
	log.Printf("setRquests done")
	return []*gnmi.SetRequest{setRequest}, nil
}

func (d *sonic) handleErrors(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-d.errc:
			return fmt.Errorf("Error in sonic provider: %v", err)
		}
	}
}

func (d *sonic) Run(ctx context.Context) error {
	// Run updatePlatform at the specified polling interval,
	// forever. PollForever sends the updates produced by
	// updatePlatorm to the gNMI client and sends any
	// resulting errors to the error channel to be handled by
	// handleErrors.
	go pgnmi.PollForever(ctx, d.client, d.pollInterval,
		d.updatePlatform, d.errc)

	// handleErrors only returns if it sees an error.
	return d.handleErrors(ctx)
}

func (d *sonic) InitGNMI(client gnmi.GNMIClient) {
	d.client = client
}

func (d *sonic) OpenConfig() bool {
	return true
}

// NewSonicProvider returns a sonic provider that registers a
// sonic device and streams interface statistics.
func NewSonicProvider(pollInterval time.Duration) provider.GNMIProvider {
	return &sonic{
		errc:         make(chan error),
		pollInterval: pollInterval,
	}
}
