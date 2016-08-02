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

// Package mock provides a mock implementation for dbmanager.  It stores
// resources in a set of public maps that test programs can use to verify
// correctness.
package mock

import (
	"log"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/types"

	"github.com/netapp/kubevoltracker/dbmanager"
	"github.com/netapp/kubevoltracker/resources"
)

type nfsID struct {
	ipAddr string
	path   string
}

type iscsiID struct {
	targetPortal string
	iqn          string
	lun          int
	fsType       string
}

type ResourceAttrs interface {
	GetName() string
	GetUID() types.UID
}

type PodAttrs struct {
	Name       string
	CreateTime unversioned.Time
	Namespace  string
	UID        types.UID
	Containers []resources.ContainerDesc
}

func (p *PodAttrs) GetName() string   { return p.Name }
func (p *PodAttrs) GetUID() types.UID { return p.UID }

type PVAttrs struct {
	Name       string
	CreateTime unversioned.Time
	NFSID      int
	ISCSIID    int
	UID        types.UID
	Storage    int64
}

func (p *PVAttrs) GetName() string   { return p.Name }
func (p *PVAttrs) GetUID() types.UID { return p.UID }

type PVCAttrs struct {
	Name       string
	CreateTime unversioned.Time
	Namespace  string
	UID        types.UID
	Storage    int64
}

func (p *PVCAttrs) GetName() string   { return p.Name }
func (p *PVCAttrs) GetUID() types.UID { return p.UID }

type MockManager struct {
	nfsIDMap    map[nfsID]int
	iscsiIDMap  map[iscsiID]int
	lastNFSID   int
	lastISCSIID int

	PodForUID map[types.UID]ResourceAttrs
	PVForUID  map[types.UID]ResourceAttrs
	PVCForUID map[types.UID]ResourceAttrs
	Deletions int

	invalidRVs bool
}

func (m *MockManager) Destroy() {
	return
}

func (m *MockManager) InsertPod(uid types.UID, name string,
	createTime unversioned.Time, namespace string,
	containers []resources.ContainerDesc,
	json, watcherNS, rv string) {

	m.PodForUID[uid] = &PodAttrs{Name: name, CreateTime: createTime,
		Namespace: namespace, Containers: containers, UID: uid}
}

func (m *MockManager) InsertPV(
	uid types.UID, name string, createTime unversioned.Time, backendID int,
	backendType dbmanager.Table, storage int64,
	accessModes []api.PersistentVolumeAccessMode, json, rv string,
) {
	nfsID := 0
	iscsiID := 0

	switch {
	case backendType == dbmanager.NFS:
		nfsID = backendID
	case backendType == dbmanager.ISCSI:
		iscsiID = backendID
	default:
		log.Fatal("Unrecognized backend type when inserting PV:  ", backendType)
	}
	m.PVForUID[uid] = &PVAttrs{Name: name, CreateTime: createTime,
		NFSID: nfsID, ISCSIID: iscsiID, Storage: storage, UID: uid}
}

func (m *MockManager) InsertPVC(uid types.UID, name string,
	createTime unversioned.Time, namespace string, storage int64,
	accessModes []api.PersistentVolumeAccessMode,
	json, watcherNS, rv string) {
	m.PVCForUID[uid] = &PVCAttrs{Name: name, CreateTime: createTime,
		Namespace: namespace, Storage: storage, UID: uid}
}

func (m *MockManager) InsertNFS(ipAddr, path string) int {
	newNFSID := nfsID{ipAddr, path}
	if id, ok := m.nfsIDMap[newNFSID]; ok {
		return id
	}
	m.lastNFSID++
	m.nfsIDMap[newNFSID] = m.lastNFSID
	return m.lastNFSID
}

func (m *MockManager) InsertISCSI(
	targetPortal, iqn string, lun int, fsType string,
) int {
	newISCSIID := iscsiID{targetPortal, iqn, lun, fsType}
	if id, ok := m.iscsiIDMap[newISCSIID]; ok {
		return id
	}
	m.lastISCSIID++
	m.iscsiIDMap[newISCSIID] = m.lastISCSIID
	return m.lastISCSIID
}

func (m *MockManager) BindPVC(pvUID types.UID, pvcUID types.UID,
	bindTime unversioned.Time, rv string) {
	return
}

func (m *MockManager) DeletePod(uid types.UID, deleteTime unversioned.Time,
	watcherNS, rv string) {
	delete(m.PodForUID, uid)
	m.Deletions++
}
func (m *MockManager) DeletePV(uid types.UID, deleteTime unversioned.Time,
	rv string) {
	delete(m.PVForUID, uid)
	m.Deletions++
}
func (m *MockManager) DeletePVC(uid types.UID, deleteTime unversioned.Time,
	watcherNS, rv string) {
	delete(m.PVCForUID, uid)
	m.Deletions++
}

func (m *MockManager) UpdatePV(
	uid types.UID, backendID int, backendType dbmanager.Table, storage int64,
	accessModes []api.PersistentVolumeAccessMode, json, rv string,
) {
	nfsID := 0
	iscsiID := 0

	switch {
	case backendType == dbmanager.NFS:
		nfsID = backendID
	case backendType == dbmanager.ISCSI:
		iscsiID = backendID
	default:
		log.Fatal("Unrecognized backend type when inserting PV:  ", backendType)
	}
	pv := m.PVForUID[uid].(*PVAttrs)
	pv.NFSID = nfsID
	pv.ISCSIID = iscsiID
	pv.Storage = storage
}

func (m *MockManager) UpdatePVC(uid types.UID, storage int64,
	accessModes []api.PersistentVolumeAccessMode,
	json, watcherNS, rv string) {
	pvc := m.PVCForUID[uid].(*PVCAttrs)
	pvc.Storage = storage
}

func (m *MockManager) GetRV(resource resources.ResourceType,
	namespace string) string {
	// TODO:  Actually emulate changing rvs over time?

	if m.invalidRVs {
		return "1"
	}
	return ""
}

func (m *MockManager) ValidateConnection() error {
	// Always succeed.
	return nil
}

func New(invalidRVs bool) dbmanager.DBManager {
	return &MockManager{
		nfsIDMap:    make(map[nfsID]int),
		iscsiIDMap:  make(map[iscsiID]int),
		lastNFSID:   0,
		lastISCSIID: 0,
		PodForUID:   make(map[types.UID]ResourceAttrs),
		PVForUID:    make(map[types.UID]ResourceAttrs),
		PVCForUID:   make(map[types.UID]ResourceAttrs),
		Deletions:   0,
		invalidRVs:  invalidRVs,
	}
}
