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
	"log"
	"os"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"

	"github.com/netapp/kubevoltracker/dbmanager"
	"github.com/netapp/kubevoltracker/dbmanager/mock"
	"github.com/netapp/kubevoltracker/kubectl"
	"github.com/netapp/kubevoltracker/resources"
)

var manager *mock.MockManager

func GetWatcherForManager(manager dbmanager.DBManager) *Watcher {
	w, err := NewWatcher(watcherNS, os.Getenv("KUBERNETES_MASTER"),
		manager)
	if err != nil {
		log.Fatal("Unable to create watcher; aborting test: ", err)
	}
	return w
}

func GetWatcher() *Watcher {
	manager = (mock.New(false)).(*mock.MockManager)
	return GetWatcherForManager(manager)
}

func GetInvalidRVWatcher() *Watcher {
	manager = (mock.New(true)).(*mock.MockManager)
	return GetWatcherForManager(manager)
}

// Since PVs aren't namespaced and we don't want to require the user to run
// the tests with a completely clean environment, allow for some extra entries
// in the map.
// For convenience, returns a map of names to UIDs
func verifyPVMapContents(t *testing.T, m map[types.UID]mock.ResourceAttrs,
	names []string) map[string]types.UID {

	ret := make(map[string]types.UID)
	if len(m) < len(names) {
		t.Errorf("Expected at least %d entries in PV map; got %d",
			len(names), len(m))
	}
	for _, n := range names {
		match := false
		for _, r := range m {
			if n == r.GetName() {
				match = true
				ret[n] = r.GetUID()
				break
			}
		}
		if !match {
			t.Errorf("PV %s not in map", n)
		}
	}
	return ret
}

// Other resources are namespaced, and since we completely control the test
// namespace, we can expect an exact match.
func verifyMapContents(t *testing.T, m map[types.UID]mock.ResourceAttrs,
	names []string, resourceName string) map[string]types.UID {

	ret := make(map[string]types.UID)
	if len(m) != len(names) {
		t.Errorf("Expected %d %s in map; got %d", len(names), resourceName,
			len(m))
	}
	for _, r := range m {
		match := false
		for _, n := range names {
			if n == r.GetName() {
				match = true
				ret[n] = r.GetUID()
				break
			}
		}
		if !match {
			t.Errorf("Incorrect name in %s:  %s\n", resourceName,
				r.GetName())
		}
	}
	return ret
}

//TODO:  Figure out a good way to test pod creation; the main issue is that
// teardown is a fairly lengthy process.

func TestWatchPV(t *testing.T) {
	var watchNFSName string
	var err error
	var pvAccessModes = []api.PersistentVolumeAccessMode{api.ReadWriteMany,
		api.ReadOnlyMany}

	kubectl.DeleteTestResources()
	w := GetWatcher()
	defer w.Destroy()

	w.Watch(resources.PVs, false)
	if watchNFSName, err = kubectl.CreatePV(
		"watch-nfs", 1, pvAccessModes,
	); err != nil {
		t.Fatalf("Unable to create %s:  %s", watchNFSName, err)
	}
	time.Sleep(time.Second)
	verifyPVMapContents(t, manager.PVForUID, []string{"watch-nfs"})
	if f, err := kubectl.CreatePV("watch-nfs-2", 1, pvAccessModes); err != nil {
		t.Fatalf("Unable to create %s:  %s", f, err)
	}
	time.Sleep(time.Second)
	verifyPVMapContents(t, manager.PVForUID,
		[]string{"watch-nfs", "watch-nfs-2"})

	if err := kubectl.RunKubectl(kubectl.Delete, watchNFSName,
		true); err != nil {
		t.Fatal("Unable to delete nfs-pv.yaml: ", err)
	}
	time.Sleep(time.Second)
	verifyPVMapContents(t, manager.PVForUID, []string{"watch-nfs-2"})
}

func TestWatchPVInit(t *testing.T) {
	kubectl.DeleteTestResources()
	w := GetWatcher()
	defer w.Destroy()
	pvNames := []string{"nfs", "nfs-2"}
	pvAccessModes := []api.PersistentVolumeAccessMode{api.ReadWriteOnce}

	for _, n := range pvNames {
		if f, err := kubectl.CreatePV(n, 1, pvAccessModes); err != nil {
			t.Fatalf("Unable to create %s:  %s", f, err)
		}
	}
	w.Watch(resources.PVs, true)
	time.Sleep(time.Second)

	verifyPVMapContents(t, manager.PVForUID, pvNames)
}

func TestPVC(t *testing.T) {

	kubectl.DeleteTestResources()
	w := GetWatcher()
	defer w.Destroy()

	w.Watch(resources.PVCs, false)
	if fileName, err := kubectl.CreatePVC(
		"nfs", watcherNS, 1,
		[]api.PersistentVolumeAccessMode{api.ReadWriteOnce},
	); err != nil {
		t.Fatalf("Unable to create %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	verifyMapContents(t, manager.PVCForUID, []string{"nfs"}, "PVCs")
}

func TestPVUpdate(t *testing.T) {
	var (
		pvName      = "nfs-update"
		initialSize = 1
		newSize     = 5
		accessModes = []api.PersistentVolumeAccessMode{api.ReadWriteOnce,
			api.ReadOnlyMany}
	)

	kubectl.DeleteTestResources()
	w := GetWatcher()
	defer w.Destroy()

	w.Watch(resources.PVs, false)
	if fileName, err := kubectl.CreatePV(pvName, initialSize, accessModes); err != nil {
		t.Fatalf("Unable to create %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	nameMap := verifyPVMapContents(t, manager.PVForUID, []string{pvName})
	r, ok := manager.PVForUID[nameMap[pvName]]
	if !ok {
		t.Fatalf("%s not in name map; test cannot run further.", pvName)
	}
	pv, ok := r.(*mock.PVAttrs)
	if !ok {
		t.Fatalf("PV %s stored as the wrong type.", pvName)
	}
	if pv.Storage != kubectl.GetStorageValue(initialSize) {
		t.Errorf("Got initial PV size %d; expected %d", pv.Storage,
			initialSize)
	}

	if fileName, err := kubectl.UpdatePV(pvName, newSize, accessModes); err != nil {
		t.Fatalf("Unable to update %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	verifyPVMapContents(t, manager.PVForUID, []string{"nfs-update"})
	if pv.Storage != kubectl.GetStorageValue(newSize) {
		t.Errorf("Got new PV size %d; expected %d", pv.Storage, newSize)
	}
}

func TestPVCUpdate(t *testing.T) {
	var (
		pvcName     = "nfs-update"
		initialSize = 1
		newSize     = 5
		accessModes = []api.PersistentVolumeAccessMode{api.ReadOnlyMany}
	)

	kubectl.DeleteTestResources()
	w := GetWatcher()
	defer w.Destroy()

	w.Watch(resources.PVCs, false)
	if fileName, err := kubectl.CreatePVC(
		pvcName, watcherNS, initialSize, accessModes,
	); err != nil {
		t.Fatalf("Unable to create %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	nameMap := verifyMapContents(t, manager.PVCForUID, []string{pvcName},
		"PVCs")
	r, ok := manager.PVCForUID[nameMap[pvcName]]
	if !ok {
		t.Fatalf("%s not in name map; test cannot run further.", pvcName)
	}
	pvc, ok := r.(*mock.PVCAttrs)
	if !ok {
		t.Fatalf("PVC %s stored as the wrong type.", pvcName)
	}
	if pvc.Storage != kubectl.GetStorageValue(initialSize) {
		t.Errorf("Got initial PVC size %d; expected %d", pvc.Storage,
			initialSize)
	}

	if fileName, err := kubectl.UpdatePVC(
		pvcName, watcherNS, newSize, accessModes,
	); err != nil {
		t.Fatalf("Unable to update %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	verifyMapContents(t, manager.PVCForUID, []string{"nfs-update"},
		"PVCs")
	if pvc.Storage != kubectl.GetStorageValue(newSize) {
		t.Errorf("Got new PVC size %d; expected %d", pvc.Storage, newSize)
	}
}

func TestStop(t *testing.T) {
	w := GetWatcher()
	w.Watch(resources.PVCs, false)
	defer w.Destroy()
	if _, ok := w.stopChannels[resources.PVCs]; !ok {
		// This should be fatal because the other tests go weird if this
		// doesn't hold
		t.Fatal("Stop channel not added to watcher map.")
	}
	if err := w.Stop(resources.Pods); err == nil {
		t.Error("Watcher failed to report an error when stopped on an",
			"unwatched resource")
	}
	if err := w.Stop(resources.PVCs); err != nil {
		t.Error("Watcher failed to stop watched channel.")
	}
	if _, ok := w.stopChannels[resources.PVCs]; ok {
		t.Error("Stop channel still in resources.")
	}
}

func TestBadWatcherParams(t *testing.T) {
	manager = (mock.New(false)).(*mock.MockManager)
	if _, err := NewWatcher(watcherNS, os.Getenv("KUBERNETES_MASTER"),
		nil); err == nil {
		t.Error("NewWatcher failed to validate nil database manager.")
	}
	if _, err := NewWatcher(watcherNS, "", manager); err == nil {
		t.Error("NewWatcher failed to validate API server address/port")
	}
}

func TestInvalidResource(t *testing.T) {
	r := resources.ResourceType("bogus")
	w := GetWatcher()
	defer w.Destroy()
	if _, err := w.getHandler(r); err == nil {
		t.Error("Expected getHandler error for resource type ", r)
	}
	if err := w.Watch(r, false); err == nil {
		t.Error("Expected Watcher.Watch() error for resource type ", r)
	}
}

func TestInvalidRV(t *testing.T) {

	var accessModes = []api.PersistentVolumeAccessMode{api.ReadWriteOnce}

	kubectl.DeleteTestResources()
	if pvName, err := kubectl.CreatePV("invalid-rv-nfs", 1, accessModes); err != nil {
		t.Fatalf("Unable to create %s:  %s", pvName, err)
	}

	// Create a PVC that gets deleted before the watcher starts to ensure that
	// the test isn't secretly reading a valid RV from somewhere (if so,
	// this RV will show up as a deletion; if it's doing the right thing, the
	// watcher will have no idea that this existed)
	deletePVCName, err := kubectl.CreatePVC("invalid-rv-to-delete",
		watcherNS, 5, accessModes)
	if err != nil {
		t.Fatalf("Unable to create %s: %s", deletePVCName, err)
	}
	if err := kubectl.RunKubectl(kubectl.Delete, deletePVCName,
		true); err != nil {
		t.Fatalf("Unable to delete %s:  %s", deletePVCName, err)
	}

	if pvcName, err := kubectl.CreatePVC(
		"invalid-rv-nfs", watcherNS, 1, accessModes,
	); err != nil {
		t.Fatalf("Unable to create %s:  %s", pvcName, err)
	}
	time.Sleep(time.Second)

	w := GetInvalidRVWatcher()
	defer w.Destroy()
	w.Watch(resources.PVs, false)
	w.Watch(resources.PVCs, false)

	time.Sleep(time.Second)
	verifyPVMapContents(t, manager.PVForUID, []string{"invalid-rv-nfs"})
	verifyMapContents(t, manager.PVCForUID, []string{"invalid-rv-nfs"}, "PVCs")
	if manager.Deletions != 0 {
		t.Errorf("Observed %d deletions; expected 0.", manager.Deletions)
	}
}
