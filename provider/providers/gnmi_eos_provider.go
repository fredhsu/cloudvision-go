// Copyright (c) 2018 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.
// Subject to Arista Networks, Inc.'s EULA.
// FOR INTERNAL USE ONLY. NOT FOR DISTRIBUTION.

package providers

import (
	"arista/provider"
	"arista/schema"
	"arista/types"
	"context"
	"fmt"

	"github.com/aristanetworks/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
)

type gnmieos struct {
	provider.ReadOnly
	prov        provider.GNMIProvider
	errc        chan error
	server      gnmi.GNMIServer
	client      gnmi.GNMIClient
	ready       chan struct{}
	initialized bool
	notifChan   chan<- types.Notification
}

func (g *gnmieos) Init(s *schema.Schema, root types.Entity,
	notification chan<- types.Notification) {
	g.notifChan = notification
	g.initialized = true
}

func (g *gnmieos) Run(ctx context.Context) error {
	if !g.initialized {
		return fmt.Errorf("Provider is uninitialized")
	}

	var err error
	ctx, g.server, err = GNMIServer(ctx, g.notifChan, g.errc)
	if err != nil {
		glog.Errorf("Error in creating GNMIServer: %v", err)
	}

	g.client = GNMIClient(g.server)
	g.prov.InitGNMI(g.client)
	close(g.ready)
	err = g.prov.Run(ctx)
	return err
}

func (g *gnmieos) WaitForNotification() {
	<-g.ready
}

// NewGNMIEOSProvider takes in a GNMIProvider and returns the same
// provider, converted to an EOSProvider
func NewGNMIEOSProvider(gp provider.GNMIProvider) provider.EOSProvider {
	return &gnmieos{
		prov:  gp,
		errc:  make(chan error),
		ready: make(chan struct{}),
	}
}
