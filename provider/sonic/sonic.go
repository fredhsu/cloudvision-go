// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package sonic

import (
	"context"
	"fmt"
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
		pgnmi.Update(pgnmi.PlatformComponentStatePath("SONiC Software Version", "software-version"),
			pgnmi.Strval("SONiC.staphylo.108945.1")),
		// pgnmi.Update(pgnmi.IntfStateCountersPath(intfName, "in-errors"),
		// 	pgnmi.Uintval(inErrs)),
	}, nil
}

func (d *sonic) updateInterfaces() ([]*gnmi.SetRequest, error) {
	setRequest := new(gnmi.SetRequest)
	updates, err := updatesFromSystem()
	if err != nil {
		return nil, err
	}

	setRequest.Delete = []*gnmi.Path{pgnmi.Path("platform")}
	setRequest.Replace = updates
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
	// Run updateInterfaces at the specified polling interval,
	// forever. PollForever sends the updates produced by
	// updateInterfaces to the gNMI client and sends any
	// resulting errors to the error channel to be handled by
	// handleErrors.
	go pgnmi.PollForever(ctx, d.client, d.pollInterval,
		d.updateInterfaces, d.errc)

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
