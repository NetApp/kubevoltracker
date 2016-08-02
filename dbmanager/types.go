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

// Package dbmanager provides a set of constants and interfaces for interacting
// with a backend data store.  Packages should generally rely on the methods
// exposed here, rather than those in implementing packages.  Similarly,
// implementing packages should keep the majority of their implementation
// private.
package dbmanager

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/types"

	"github.com/netapp/kubevoltracker/resources"
)

type Table string

const (
	Pod       Table = "pod"
	PVC       Table = "pvc"
	PV        Table = "pv"
	NFS       Table = "nfs"
	ISCSI     Table = "iscsi"
	PodMount  Table = "pod_mount"
	Container Table = "container"
)

type DBManager interface {
	Destroy()
	// ValidateConnection waits to establish that the database is online and
	// returns an error if it fails to connect in a timely manner.
	ValidateConnection() error
	// InsertPod adds a new Pod resource to the backend state.
	InsertPod(uid types.UID, name string, createTime unversioned.Time,
		namespace string, containers []resources.ContainerDesc, json,
		watcherNS, rv string)
	// InsertPV adds a new Persistent Volume resource to the backend state
	// backendID should be an ID returned by InsertNFS or InsertISCSI,
	// and backendType should be the table to which that ID belongs.
	InsertPV(uid types.UID, name string, createTime unversioned.Time,
		backendID int, backendType Table, storage int64,
		accessModes []api.PersistentVolumeAccessMode,
		json, rv string)
	// InsertPVC adds a new Persistent Volume Claim to the backend state
	InsertPVC(uid types.UID, name string, createTime unversioned.Time,
		namespace string, storage int64,
		accessModes []api.PersistentVolumeAccessMode,
		json, watcherNS, rv string)
	// InsertNFS checks whether the specified IP Address and path correspond
	// to a known NFS backend.  If so, it returns the ID for for that backend.
	// If not, it inserts a new record for it and returns the newly created ID.
	InsertNFS(ipAddr, path string) int
	// InsertISCSI checks whether the specified ISCSI parameters correspond
	// to a known ISCSI backend.  If so, it returns the ID for that backend;
	// if not, it inserts a new record for it and returns the newly created ID.
	InsertISCSI(targetPortal, iqn string, lun int, fsType string) int

	// UpdatePV updates an existing PV record specified by uid, replacing the
	// original field values with those provided in the parameters.
	UpdatePV(uid types.UID, backendID int, backendType Table, storage int64,
		accessModes []api.PersistentVolumeAccessMode, json,
		rv string)
	// UpdatePVC updates an existing PVC record specified by uid, replacing the
	// original field values with those provided in the parameters.
	UpdatePVC(uid types.UID, storage int64,
		accessModes []api.PersistentVolumeAccessMode,
		json, watcherNS, rv string)

	// BindPVC records a binding between the PV and PVC whose UIDs are specified
	// in the parameters.
	BindPVC(pvUID types.UID, pvcUID types.UID, bindTime unversioned.Time,
		rv string)

	// DeletePod records the time a Pod was deleted.
	DeletePod(uid types.UID, deleteTime unversioned.Time, watcherNS,
		rv string)
	// DeletePV records the time a PV was deleted.
	DeletePV(uid types.UID, deleteTime unversioned.Time, rv string)
	// DeletePVC records the time a PVC was deleted.
	DeletePVC(uid types.UID, deleteTime unversioned.Time, watcherNS,
		rv string)

	// GetRV returns the most recent resource version for the given resource
	// type in the given namespace.  This can be used to resume resource watches
	// from the last observed point.
	GetRV(resource resources.ResourceType, namespace string) string
}
