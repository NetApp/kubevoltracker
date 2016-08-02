/*
   Copyright 2016 Chris Dragga <cdragga@netapp.com>

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/netapp/kubevoltracker/resources"
)

type EventFactory func() interface{}

// Copied from pkg/watch/watch.go; TODO:  Put this elsewhere?
type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	Error    EventType = "ERROR"

	StatusTooOld = 410
)

// PodEvent decodes a Pod API watch.  Implements ResourceEvent.
type PodEvent struct {
	Type     EventType `json:"type"`
	Resource api.Pod   `json:"object"`
}

// PodEvent decodes a Persistent Volume Claim API watch.  Implements
// ResourceEvent.
type PVCEvent struct {
	Type     EventType                 `json:"type"`
	Resource api.PersistentVolumeClaim `json:"object"`
}

// PodEvent decodes a Persistent Volume API watch.  Implements ResourceEvent.
type PVEvent struct {
	Type     EventType            `json:"type"`
	Resource api.PersistentVolume `json:"object"`
}

// Status message decodes an error message sent by the Kubernetes API server.
// This DOES NOT implement ResourceEvent.
type StatusMessage struct {
	Type   EventType          `json:"type"`
	Status unversioned.Status `json:"object"`
}

/* TODO:  Replace the following with reflection?  See below for details:
http://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-runtime-in-go */

// ResourceFactory stores a basic constructor for the different types of
// ResourceEvents.
type ResourceFactory func() ResourceEvent

// resourceFactoryMap maps a given resource type to its appropriate factory.
/* TODO: This may be kind of an anti-pattern.  Use subtypes of APIClient? */
var resourceFactoryMap = map[resources.ResourceType]ResourceFactory{
	resources.Pods: func() ResourceEvent { return new(PodEvent) },
	resources.PVs:  func() ResourceEvent { return new(PVEvent) },
	resources.PVCs: func() ResourceEvent { return new(PVCEvent) },
}

// ResourceEvent provides an abstraction around the different event types so
// that we can unify their handling.
type ResourceEvent interface {
	GetResource() resources.Resource
	GetType() EventType
	String() string
}

/* TODO:  Reduce the copying that happens here by changing stuff to pointers. */
func (p *PodEvent) GetResource() resources.Resource {
	return &resources.PodResource{Pod: p.Resource}
}
func (p *PVEvent) GetResource() resources.Resource {
	return &resources.PVResource{PersistentVolume: p.Resource}
}
func (p *PVCEvent) GetResource() resources.Resource {
	return &resources.PVCResource{PersistentVolumeClaim: p.Resource}
}

func (p *PodEvent) GetType() EventType { return p.Type }
func (p *PVEvent) GetType() EventType  { return p.Type }
func (p *PVCEvent) GetType() EventType { return p.Type }

// TODO:  It really seems like some kind of type embedding or something
// could prevent this repetition, but I think that's more trouble than it's
// worth.
func (p *PodEvent) String() string {
	return fmt.Sprintf("%s, Resource:  %s", p.Type, p.GetResource().String())
}

func (p *PVEvent) String() string {
	return fmt.Sprintf("%s, Resource:  %s", p.Type, p.GetResource().String())
}

func (p *PVCEvent) String() string {
	return fmt.Sprintf("%s, Resource:  %s", p.Type, p.GetResource().String())
}
