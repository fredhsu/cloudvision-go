// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package device

import (
	"context"
	"sync"
	"time"

	"github.com/aristanetworks/cloudvision-go/provider"
	"github.com/aristanetworks/cloudvision-go/version"
	"github.com/aristanetworks/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/metadata"
)

// An Inventory maintains a set of devices.
type Inventory interface {
	Add(key string, device Device) error
	Delete(key string) error
	Get(key string) (Device, bool)
}

// deviceConn contains a device and its gNMI connections.
type deviceConn struct {
	device            Device
	ctx               context.Context
	cancel            context.CancelFunc
	rawGNMIClient     gnmi.GNMIClient
	wrappedGNMIClient *gNMIClientWrapper
	providerGroup     *errgroup.Group
}

// inventory implements the Inventory interface.
type inventory struct {
	ctx           context.Context
	group         *errgroup.Group
	rawGNMIClient gnmi.GNMIClient
	devices       map[string]*deviceConn
	lock          sync.Mutex
}

func (dc *deviceConn) sendPeriodicUpdates() error {
	ticker := time.NewTicker(time.Second)
	ctx := metadata.AppendToOutgoingContext(dc.ctx,
		collectorVersionMetadata, version.Version)
	if _, ok := dc.device.(Manager); ok {
		// ManagementSystem is a system managing other devices which itself
		// shouldn't be treated as an actual streaming device in CloudVision.
		ctx = metadata.AppendToOutgoingContext(ctx,
			deviceTypeMetadata, "managementSystem")
	} else {
		// Target is an ordinary device streaming to CloudVision.
		ctx = metadata.AppendToOutgoingContext(ctx,
			deviceTypeMetadata, "target")
	}
	dc.wrappedGNMIClient.Set(ctx, &gnmi.SetRequest{})
	for {
		select {
		case <-dc.ctx.Done():
			return nil
		case <-ticker.C:
			if alive, err := dc.device.Alive(); err == nil {
				if alive {
					ctx := metadata.AppendToOutgoingContext(dc.ctx,
						deviceLivenessMetadata, "true")
					dc.wrappedGNMIClient.Set(ctx, &gnmi.SetRequest{})
				} else {
					did, _ := dc.device.DeviceID()
					glog.V(2).Infof("Device %s is not alive", did)
				}
			} else {
				return err
			}
		}
	}
}

func (dc *deviceConn) handleErrors() error {
	return dc.providerGroup.Wait()
}

// Add adds a device to the inventory, opens up any gNMI connections
// required by the device's providers, and then starts its providers.
func (i *inventory) Add(key string, device Device) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	if _, ok := i.devices[key]; ok {
		return nil
	}

	dc := &deviceConn{device: device}
	ctx, cancel := context.WithCancel(i.ctx)
	dc.providerGroup, dc.ctx = errgroup.WithContext(ctx)
	dc.cancel = cancel

	i.devices[key] = dc

	providers, err := device.Providers()
	if err != nil {
		return err
	}
	dc.rawGNMIClient = i.rawGNMIClient
	dc.wrappedGNMIClient = newGNMIClientWrapper(dc.rawGNMIClient, nil,
		key, false)

	for _, p := range providers {
		pt, ok := p.(provider.GNMIProvider)
		if !ok {
			return errors.New("unexpected provider type; need GNMIProvider")
		}

		pt.InitGNMI(newGNMIClientWrapper(dc.rawGNMIClient, pt, key, pt.OpenConfig()))

		// Start the providers.
		dc.providerGroup.Go(func() error {
			return p.Run(dc.ctx)
		})
	}

	// Watch for provider errors in the provider errgroup and
	// propagate them up to the inventory errgroup.
	if len(providers) != 0 {
		i.group.Go(func() error {
			return dc.handleErrors()
		})
	}

	// Send periodic updates of device-level metadata.
	i.group.Go(func() error {
		return dc.sendPeriodicUpdates()
	})

	glog.V(2).Infof("Added device %s", key)
	return nil
}

func (i *inventory) Delete(key string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	dc, ok := i.devices[key]
	if !ok {
		return nil
	}

	// Cancel the device context and delete the device from the device
	// map. We don't have to worry about propagating errors up to the
	// inventory errgroup, since handleErrors will do that. We just need
	// to make sure this device's providers are finished before deleting
	// the device.
	dc.cancel()
	_ = dc.providerGroup.Wait()
	delete(i.devices, key)
	glog.V(2).Infof("Deleted device %s", key)
	return nil
}

func (i *inventory) Get(key string) (Device, bool) {
	i.lock.Lock()
	defer i.lock.Unlock()
	d, ok := i.devices[key]
	if !ok {
		return nil, ok
	}
	return d.device, ok
}

// NewInventory creates an Inventory.
func NewInventory(ctx context.Context, group *errgroup.Group,
	gnmiClient gnmi.GNMIClient) Inventory {
	inv := &inventory{
		ctx:           ctx,
		devices:       make(map[string]*deviceConn),
		group:         group,
		rawGNMIClient: gnmiClient,
	}
	return inv
}
