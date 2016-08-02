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

// Package resources contains constants and data structures corresponding
// to the different resources defined in the Kubernetes API.
package resources

import (
	"k8s.io/kubernetes/pkg/api"
)

// ResourceType corresponds to the different API server endpoints (and,
// correspondingly, resources) that we care about.
type ResourceType string

const (
	Pods ResourceType = "pods"
	PVs  ResourceType = "persistentvolumes"
	PVCs ResourceType = "persistentvolumeclaims"
)

// Persistent volumes are not namespaced, so use the default namespace.
const PVNamespace = "default"

// VolumeMount represents a PVC mount point for a container.
type VolumeMount struct {
	Name     string
	ReadOnly bool
}

// ContainerDesc provides an abstraction that allows us to pass around the
// relevant parts of a container more easily.
type ContainerDesc struct {
	Name      string
	Image     string
	Command   string
	PVCMounts []VolumeMount
	// TODO:  Include mount path?
}

// PodResource wraps an api.Pod and implements the Resource interface.
type PodResource struct {
	api.Pod
}

// PVResource wraps an api.PersistentVolume and implements the Resource
// interface.
type PVResource struct {
	api.PersistentVolume
}

// PVResource wraps an api.PersistentVolumeClaim and implements the Resource
// interface.
type PVCResource struct {
	api.PersistentVolumeClaim
}
