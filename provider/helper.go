// Copyright (c) 2016 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.
// Subject to Arista Networks, Inc.'s EULA.
// FOR INTERNAL USE ONLY. NOT FOR DISTRIBUTION.

package provider

import (
	"fmt"
	"time"

	"arista/types"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/path"
)

// NotificationsForInstantiateChild this is a helper method for
// Providers to use to generate the notifications associated with
// instantiating a child
func NotificationsForInstantiateChild(ts time.Time, child types.Entity, attrDef *types.AttrDef,
	k key.Key) []types.Notification {
	var notifs []types.Notification
	def := child.GetDef()
	if def.IsDirectory() {
		// If we just created a directory, just send one notification
		// to delete-all the new directory, instead of sending the
		// directory's attributes, which are internal.
		notifs = make([]types.Notification, 2)
		notifs[0] = types.NewNotificationWithEntity(ts, child.Path(), []key.Key{}, nil,
			child)
	} else {
		p := child.Path()
		initialAttrs := make(map[key.Key]interface{}, len(def.Attrs))
		var deletePaths []path.Path
		for _, i := range def.AttrsOrderByID {
			attrName := def.AttributesByID[i].Name
			v, _ := child.GetAttribute(attrName)
			attrKey := key.New(attrName)
			if _, ok := v.(types.Collection); ok {
				// Transform any collection into a pointer.
				childPath := path.Append(p, attrName)
				initialAttrs[attrKey] = types.NewPointer(childPath)
				deletePaths = append(deletePaths, childPath)
			} else {
				initialAttrs[attrKey] = v
			}
		}
		notifs = make([]types.Notification, 2+len(deletePaths))
		notifs[0] = types.NewNotificationWithEntity(ts, p, nil, initialAttrs, child)
		for i, deletePath := range deletePaths {
			// Leave the first 2 notifications as they are since they're swapped later depending on
			// which mode we're in.
			notifs[i+2] = types.NewNotification(ts, deletePath, []key.Key{}, nil)
		}
	}
	parent := child.Parent()
	attrName := attrDef.Name
	if k == nil { // Regular attribute
		notifs[1] = types.NewNotificationWithEntity(ts, parent.Path(), nil,
			map[key.Key]interface{}{key.New(attrName): child.Ptr()}, parent)
	} else { // Collection
		// The path to notify on is the path of the entity + "/" + the
		// collection name, *except* if we're adding an entry to a directory.
		p := parent.Path()
		if !parent.GetDef().IsDirectory() {
			p = path.Append(p, attrName)
		}
		notifs[1] = types.NewNotificationWithEntity(ts, p, nil,
			map[key.Key]interface{}{k: child.Ptr()}, parent)
	}
	// In "AgentMode" the ordering of the two notifications should be switched
	if GetMode() == AgentMode {
		notifs[0], notifs[1] = notifs[1], notifs[0]
	}
	return notifs
}

// NotificationsForDeleteChild is a helper for Providers. It returns
// the notifs that should be sent when an entity is deleted.
func NotificationsForDeleteChild(ts time.Time, child types.Entity, attrDef *types.AttrDef,
	k key.Key) ([]types.Notification, error) {
	parent := child.Parent()
	if parent == nil {
		return nil, fmt.Errorf("Can't generate notifications: %s",
			NewErrParentIsNil(child.Path()))
	}

	p := parent.Path()
	if attrDef.IsColl {
		// Use path to collection
		p = path.Append(p, attrDef.Name)
	} else {
		// Key is attribute name
		k = key.New(attrDef.Name)
	}

	notifs, err := recursiveEntityDeleteNotification(nil, child, child.GetDef(), ts)
	if err != nil {
		return notifs, fmt.Errorf("Error recursively deleting entities with"+
			" notifications under %q: %s",
			child.Path(), err)
	}

	// Zero out the child's attributes.
	notifs = append(notifs, types.NewNotificationWithEntity(ts, child.Path(),
		[]key.Key{}, nil, child))

	// Finally remove this entity from its parent's attribute or collection
	notifs = append(notifs, types.NewNotificationWithEntity(ts, p, []key.Key{k}, nil, parent))

	return notifs, nil
}

// recursiveEntityDeleteNotification recursively walks down a deleted entity,
// looking for and deleting any child instantiating attributes that hold entities.
// notifs is appended to and returned.
func recursiveEntityDeleteNotification(notifs []types.Notification, e types.Entity,
	def *types.TypeDef, ts time.Time) ([]types.Notification, error) {
	if !def.TypeFlags.IsEntity {
		// Should be impossible, as it would imply something wrong with the schema
		panic(fmt.Sprintf("Found an entity %#v at path %s with isEntity=false in typeDef: %#v",
			e, e.Path(), def))
	}

	var childEntities []types.Entity
	// afterRecurseNotifs are notifs that should be added after the
	// recursive call
	var afterRecurseNotifs []types.Notification
	for _, i := range def.AttrsOrderByID {
		attr := def.AttributesByID[i]
		if !attr.IsInstantiating {
			if attr.IsColl {
				afterRecurseNotifs = append(afterRecurseNotifs, types.NewNotificationWithEntity(ts,
					path.Append(e.Path(), attr.Name), []key.Key{}, nil, e))
			}
			continue
		}
		if attr.IsColl {
			afterRecurseNotifs = append(afterRecurseNotifs, types.NewNotificationWithEntity(ts,
				path.Append(e.Path(), attr.Name), []key.Key{}, nil, e))
			children := e.GetCollection(attr.Name)
			children.ForEach(func(k key.Key, child interface{}) error {
				childEntities = append(childEntities, child.(types.Entity))
				return nil
			})
		} else if child, ok := e.GetEntity(attr.Name); ok {
			childEntities = append(childEntities, child)
		}
	}
	// For every child entity we found, we recursively call ourselves to look
	// for more child entities that need to be deleted, and then call
	// types.NewNotificationWithEntity to send notification regarding those
	// deleted entities
	for _, childEntity := range childEntities {
		var err error
		notifs, err = recursiveEntityDeleteNotification(notifs, childEntity,
			childEntity.GetDef(), ts)
		if err != nil {
			return notifs, fmt.Errorf("Error recursively deleting entities with"+
				"notifications under %q: %s",
				childEntity.Path(), err)
		}
		notifs = append(notifs, types.NewNotificationWithEntity(ts,
			childEntity.Path(), []key.Key{}, nil, childEntity))
	}
	return append(notifs, afterRecurseNotifs...), nil
}

// NotificationsForCollectionCount is a helper method for Providers to use to
// generate the notifications associated with collection counts.
func NotificationsForCollectionCount(ts time.Time, collPath path.Path, count uint32,
	parent types.Entity) types.Notification {
	if GetMode() != StreamingMode || parent.GetDef().IsDirectory() {
		return nil
	}

	return types.NewNotificationWithEntity(ts, path.Append(path.Parent(collPath), "_counts"),
		nil, map[key.Key]interface{}{path.Base(collPath): count}, parent)
}
