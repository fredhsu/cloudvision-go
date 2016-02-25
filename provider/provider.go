// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.
// Subject to Arista Networks, Inc.'s EULA.
// FOR INTERNAL USE ONLY. NOT FOR DISTRIBUTION.

package provider

import (
	"arista/schema"
	"arista/types"
	"fmt"

	"github.com/aristanetworks/goarista/key"
)

// A Provider "owns" certain entities.  There are providers for entities
// coming from different sources (from Sysdb, from Smash, from /proc, etc.).
// Providers typically run in their own Goroutine(s), e.g. to read from the
// socket from Sysdb or from the shared memory files for Smash.  Providers can
// be asked to stop.  They also have a method used to write an update back to
// the source (e.g. send a message to Sysdb or update a shared-memory file for
// Smash).  Some providers can be read-only (e.g. the provider exposing data
// from /proc).
type Provider interface {
	// Run() kicks off the provider.  This method does not return until Stop()
	// is invoked, and is thus usually invoked by doing `go provider.Run()'.
	Run(s *schema.Schema, root types.Entity, notification chan<- types.Notification)

	// WaitForNotification() waits for a provider to be able to send on the notification channel
	WaitForNotification()

	// Stop() asks the provider to stop executing and clean up any Goroutines
	// it has started and release any resources it had acquired.
	// The provider will then stop, asynchronously.
	Stop()

	// Write asks the provider to apply the updates carried by the given
	// Notification to its data source (e.g. by sending an update to Sysdb
	// or updating a Smash table, etc.).  The error is returned asynchronsouly.
	// relatedEntities is a map of entities that are related to this write that
	// are not held inside the notif.  For example, in a delete, relatedEntities
	// contains the entity being deleted.
	// (relatedEntities has to be a map[key.Key]interface{} to work with the
	// Key.SetToMap and Key.GetFromMap funcs)
	Write(notif types.Notification, result chan<- error, relatedEntities map[key.Key]interface{})

	// InstantiateChild asks the provider to instantiate the new child
	// entity in the provider's data source.  k is the key in the
	// parent's collection that this entity is being instantiated
	// in. If the entity is not part of a collection k should be nil.
	// Can return ErrParentNotFound.
	InstantiateChild(child types.Entity, attrDef *schema.AttrDef,
		k key.Key, ctorArgs map[string]interface{}) error
}

// ErrParentNotFound comes from InstantiateChild when the child's
// parent is unknown.
type ErrParentNotFound struct {
	childPath  string
	parentPath string
}

// NewErrParentNotFound creates a new ErrParentNotFound
func NewErrParentNotFound(childPath string, parentPath string) error {
	return &ErrParentNotFound{
		childPath:  childPath,
		parentPath: parentPath}
}

func (e *ErrParentNotFound) Error() string {
	return fmt.Sprintf("Parent of %s (%s) not found", e.childPath, e.parentPath)
}

// IsParentNotFound tells you if an error is a ErrParentNotFound
func IsParentNotFound(err error) bool {
	_, ok := err.(*ErrParentNotFound)
	return ok
}
