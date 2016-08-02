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

package mysql

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/types"

	"github.com/netapp/kubevoltracker/dbmanager"
	tu "github.com/netapp/kubevoltracker/dbmanager/mysql/testutils"
	"github.com/netapp/kubevoltracker/resources"
)

func ValidateContainerCount(t *testing.T, podUID types.UID, expected int,
	name string) {

	correct := tu.ValidateResult(t,
		"SELECT COUNT(*) FROM container WHERE pod_uid LIKE '"+string(podUID)+
			"'",
		[]interface{}{expected},
		[]reflect.Type{tu.IntType},
	)
	if !correct {
		t.Errorf("Wrong number of containers created for %s\n", name)
	} else {
		t.Logf("Correct number of containers created for %s\n", name)
	}
}

func ValidateContainer(t *testing.T, container resources.ContainerDesc,
	podUID types.UID, name string) {

	correct := tu.ValidateResult(t,
		"SELECT image, command FROM container WHERE pod_uid LIKE '"+
			string(podUID)+"' AND name LIKE '"+container.Name+"'",
		[]interface{}{container.Image, container.Command},
		[]reflect.Type{tu.StringType, tu.StringType},
	)
	if !correct {
		t.Errorf("%s not created for pod", name)
	} else {
		t.Logf("%s created for pod.", name)
	}
}

func TestInsert(t *testing.T) {
	manager.clearTestTables()

	rv := 500

	nfs_id := manager.InsertNFS(nfs_server, nfs_path)
	if nfs_id <= 0 {
		t.Error("Invalid ID received during insert:  ", nfs_id)
	}
	nfs_query := fmt.Sprintf("SELECT id, inet_ntoa(ip_addr), path FROM nfs"+
		" WHERE id = %d", nfs_id)
	correct := tu.ValidateResult(t, nfs_query,
		[]interface{}{nfs_id, nfs_server, nfs_path},
		[]reflect.Type{tu.IntType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("NFS insert not validated")
	}

	pv_time := unversioned.Now()
	manager.InsertPV(pv_uid, pv_name, pv_time, nfs_id, dbmanager.NFS,
		pv_storage, pv_access_modes, pv_json, strconv.Itoa(rv))
	correct = tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			" nfs_id, iscsi_id, storage, access_modes, json FROM pv WHERE uid "+
			"LIKE '"+pv_uid+"'",
		[]interface{}{pv_uid, pv_name, pv_time.Time, nil, nfs_id, 0, pv_storage,
			GetAccessModeString(pv_access_modes), pv_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.IntType, tu.IntType, tu.Int64Type, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("PV insert not validated")
	} else {
		t.Log("PV insert validated")
	}
	tu.ValidateResourceVersion(t, resources.PVs, resources.PVNamespace,
		strconv.Itoa(rv))
	rv++

	pvc_time := unversioned.Now()
	manager.InsertPVC(pvc_uid, pvc_name, pvc_time, test_ns, pvc_storage,
		pvc_access_modes, pvc_json, watcher_ns, strconv.Itoa(rv))
	correct = tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			" pv_uid, namespace, storage, access_modes, json FROM pvc WHERE "+
			"uid LIKE '"+pvc_uid+"'",
		[]interface{}{pvc_uid, pvc_name, pvc_time.Time, nil, "", test_ns,
			pvc_storage, GetAccessModeString(pvc_access_modes), pvc_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.StringType, tu.StringType, tu.Int64Type, tu.StringType,
			tu.StringType},
	)
	if !correct {
		t.Error("PVC insert not validated")
	} else {
		t.Log("PVC insert validated")
	}
	tu.ValidateResourceVersion(t, resources.PVCs, watcher_ns, strconv.Itoa(rv))
	rv++

	// Insert a pod entry without any PVC mounts
	pod_time := unversioned.Now()
	manager.InsertPod(pod_uid, pod_name, pod_time, test_ns,
		[]resources.ContainerDesc{container1, container2}, pod_json,
		watcher_ns, strconv.Itoa(rv))
	correct = tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			"namespace, json FROM pod WHERE uid LIKE '"+pod_uid+"'",
		[]interface{}{pod_uid, pod_name, pod_time.Time, nil, test_ns,
			pod_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Pod insert not validated")
	} else {
		t.Log("Pod insert validated")
	}
	// Check that no entry was created in pod_mount; note that this test
	// will fail only if InsertPod is actively misbehaving.
	correct = tu.ValidateResult(t, "SELECT COUNT(*) FROM pod_mount WHERE "+
		"pod_uid LIKE '"+pod_uid+"'", []interface{}{0},
		[]reflect.Type{tu.IntType})
	if !correct {
		t.Error("Pod without volume created one or more entries in pod_mount.")
	} else {
		t.Log("No entries in pod mount created for pod without volume")
	}
	// Check container entries.
	ValidateContainerCount(t, pod_uid, 2, "Pod")
	ValidateContainer(t, container1, pod_uid, "First container")
	ValidateContainer(t, container2, pod_uid, "Second container")
	tu.ValidateResourceVersion(t, resources.Pods, watcher_ns, strconv.Itoa(rv))
	rv++

	vol_pod_time := unversioned.Now()
	manager.InsertPod(vol_pod_uid, vol_pod_name, vol_pod_time, test_ns_alt,
		[]resources.ContainerDesc{volContainer1}, vol_pod_json,
		watcher_ns_alt, strconv.Itoa(rv))
	correct = tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time, namespace, json FROM pod "+
			"WHERE uid LIKE '"+vol_pod_uid+"'",
		[]interface{}{vol_pod_uid, vol_pod_name, vol_pod_time.Time, nil,
			test_ns_alt, vol_pod_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Pod insert with PVC not validated")
	} else {
		t.Log("Pod insert with PVC validated")
	}
	// Check that no entry was created in pod_mount; note that this test
	// will fail only if InsertPod is actively misbehaving.
	correct = tu.ValidateResult(t, "SELECT COUNT(*) FROM pod_mount WHERE "+
		"pod_uid LIKE '"+vol_pod_uid+"'", []interface{}{1},
		[]reflect.Type{tu.IntType},
	)
	if !correct {
		t.Error("Pod with volume has incorrect number of entries in pod_mount.")
	} else {
		t.Log("Pod with volume has correct number of entries.")
	}
	correct = tu.ValidateResult(t,
		"SELECT pod_uid, pvc_uid, container_name, pvc_name, read_only FROM "+
			"pod_mount WHERE pod_uid LIKE '"+vol_pod_uid+"'",
		[]interface{}{vol_pod_uid, pvc_uid, volContainer1.Name, pvc_name,
			readOnlyDefault},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType,
			tu.StringType, tu.BoolType},
	)
	if !correct {
		t.Error("Incorrect pvc_uid for pod mount.")
	} else {
		t.Log("Pod mount has correct pvc")
	}
	ValidateContainerCount(t, vol_pod_uid, 1, "Vol pod")
	ValidateContainer(t, volContainer1, vol_pod_uid, "Vol pod container")
	tu.ValidateResourceVersion(t, resources.Pods, watcher_ns_alt,
		strconv.Itoa(rv))
	rv++
}

// Test that InsertPod finds the PVC with the correct time to generate
// the mount.  Note that, despite appearances, this *can* actually happen,
// even with a well-behaved user (the PVC create/delete events may arrive
// and get processed before the pod create/delete events).
func TestMatchPodMount(t *testing.T) {
	manager.clearTestTables()
	pvc_old_time := unversioned.Now()
	pvc_current_time := unversioned.NewTime(pvc_old_time.Add(time.Second))
	pvc_other_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 2))
	pvc_future_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 4))
	// The pod gets created after current_pvc, but before future_pvc
	pod_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 3))

	manager.InsertPVC(pvc_old_uid, pod_mount_pvc_name, pvc_old_time, test_ns,
		pvc_storage, pvc_access_modes, pvc_old_json, watcher_ns, "5")
	manager.InsertPVC(pvc_current_uid, pod_mount_pvc_name, pvc_current_time,
		test_ns, pvc_storage, pvc_access_modes, pvc_current_json, watcher_ns, "6")
	manager.InsertPVC(pvc_future_uid, pod_mount_pvc_name, pvc_future_time,
		test_ns, pvc_storage, pvc_access_modes, pvc_future_json, watcher_ns, "7")
	// Insert an alternate PVC at the same time as the current one.
	manager.InsertPVC(pvc_concurrent_uid, pod_mount_pvc_concurrent_name,
		pvc_current_time, test_ns, pvc_storage, pvc_access_modes, pvc_other_json, watcher_ns, "8")
	// Insert an alternate PVC after the current one, but before the pod starts.
	manager.InsertPVC(pvc_other_uid, pod_mount_pvc_other_name, pvc_other_time,
		test_ns, pvc_storage, pvc_access_modes, pvc_other_json, watcher_ns, "9")

	manager.InsertPod(pod_mount_uid, pod_mount_name, pod_time, test_ns,
		[]resources.ContainerDesc{podMountContainer}, pod_mount_json,
		watcher_ns, "132")
	correct := tu.ValidateResult(t, "SELECT pod_uid, pvc_uid, pvc_name FROM "+
		"pod_mount WHERE pod_uid LIKE '"+pod_mount_uid+"'",
		[]interface{}{pod_mount_uid, pvc_current_uid, pod_mount_pvc_name},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Insert failed to find the correct PVC.")
	}
}

func TestLatePodMount(t *testing.T) {
	manager.clearTestTables()
	pvc_old_time := unversioned.Now()
	pvc_current_time := unversioned.NewTime(pvc_old_time.Add(time.Second))

	// The pod gets created after current_pvc, but before future_pvc
	pod_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 2))

	manager.InsertPod(pod_mount_uid, pod_mount_name, pod_time, test_ns,
		[]resources.ContainerDesc{podMountContainer}, pod_mount_json,
		watcher_ns, "431")
	correct := tu.ValidateResult(t, "SELECT pod_uid, pvc_uid, pvc_name FROM "+
		"pod_mount WHERE pod_uid LIKE '"+pod_mount_uid+"'",
		[]interface{}{pod_mount_uid, "", pod_mount_pvc_name},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Initial pod mount state incorrect.")
	}

	manager.InsertPVC(pvc_old_uid, pod_mount_pvc_name, pvc_old_time,
		test_ns_alt, pvc_storage, pvc_access_modes, pvc_old_json, watcher_ns_alt,
		"5")
	manager.InsertPVC(pvc_current_uid, pod_mount_pvc_name, pvc_current_time,
		test_ns_alt, pvc_storage, pvc_access_modes, pvc_current_json,
		watcher_ns_alt, "6")
	// Insert an alternate PVC at the same time as the current one.
	manager.InsertPVC(pvc_other_uid, pod_mount_pvc_other_name,
		pvc_current_time, test_ns_alt, pvc_storage, pvc_access_modes,
		pvc_other_json, watcher_ns_alt, "8")
	correct = tu.ValidateResult(t, "SELECT pod_uid, pvc_uid, pvc_name FROM "+
		"pod_mount WHERE pod_uid LIKE '"+pod_mount_uid+"'",
		[]interface{}{pod_mount_uid, pvc_current_uid, pod_mount_pvc_name},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Final pod mount state incorrect.")
	}
}

func TestLatePVCMount(t *testing.T) {
	manager.clearTestTables()
	pvc_old_time := unversioned.Now()
	pvc_old_delete := unversioned.NewTime(pvc_old_time.Add(time.Second * 1))
	pvc_current_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 3))

	// The pod gets created before current_pvc, but after pvc_old is deleted.
	pod_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 2))

	manager.InsertPVC(pvc_old_uid, pod_mount_pvc_name, pvc_old_time, test_ns,
		pvc_storage, pvc_access_modes, pvc_old_json, watcher_ns, "5")
	manager.DeletePVC(pvc_old_uid, pvc_old_delete, watcher_ns, "6")
	manager.InsertPVC(pvc_current_uid, pod_mount_pvc_name, pvc_current_time,
		test_ns_alt, pvc_storage, pvc_access_modes, pvc_current_json,
		watcher_ns_alt, "7")
	manager.InsertPod(pod_mount_uid, pod_mount_name, pod_time, test_ns,
		[]resources.ContainerDesc{podMountContainer}, pod_mount_json,
		watcher_ns, "8")
	correct := tu.ValidateResult(t, "SELECT pod_uid, pvc_uid, pvc_name "+
		"FROM pod_mount WHERE pod_uid LIKE '"+pod_mount_uid+"'",
		[]interface{}{pod_mount_uid, pvc_current_uid, pod_mount_pvc_name},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Late-added PVC incorrect.")
	}
}

// Similar to TestLatePVCMount, but the pod arrives before the PVC.
func TestLatePVCLatePodMount(t *testing.T) {
	manager.clearTestTables()
	pvc_old_time := unversioned.Now()
	pvc_old_delete := unversioned.NewTime(pvc_old_time.Add(time.Second * 1))
	pvc_current_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 3))

	// The pod gets created before current_pvc, but after pvc_old is deleted.
	pod_time := unversioned.NewTime(pvc_old_time.Add(time.Second * 2))

	manager.InsertPod(pod_mount_uid, pod_mount_name, pod_time, test_ns,
		[]resources.ContainerDesc{podMountContainer}, pod_mount_json,
		watcher_ns, "8")
	manager.InsertPVC(pvc_old_uid, pod_mount_pvc_name, pvc_old_time, test_ns,
		pvc_storage, pvc_access_modes, pvc_old_json, watcher_ns, "5")
	manager.DeletePVC(pvc_old_uid, pvc_old_delete, watcher_ns, "6")
	manager.InsertPVC(pvc_current_uid, pod_mount_pvc_name, pvc_current_time,
		test_ns_alt, pvc_storage, pvc_access_modes, pvc_current_json,
		watcher_ns_alt, "7")
	correct := tu.ValidateResult(t, "SELECT pod_uid, pvc_uid, pvc_name "+
		"FROM pod_mount WHERE pod_uid LIKE '"+pod_mount_uid+"'",
		[]interface{}{pod_mount_uid, pvc_current_uid, pod_mount_pvc_name},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Late-added PVC incorrect.")
	}
}

func TestInsertISCSI(t *testing.T) {
	manager.clearTestTables()

	rv := 7000

	iscsiID := manager.InsertISCSI(iscsiPortal, iscsiIQN, iscsiLUN, iscsiFSType)
	correct := tu.ValidateResult(t,
		fmt.Sprintf("SELECT target_portal, iqn, lun, fs_type FROM iscsi "+
			"WHERE id = %d", iscsiID),
		[]interface{}{iscsiPortal, iscsiIQN, iscsiLUN, iscsiFSType},
		[]reflect.Type{tu.StringType, tu.StringType, tu.IntType, tu.StringType},
	)
	if !correct {
		t.Error("ISCSI entry not added properly.")
	}

	duplicateISCSI := manager.InsertISCSI(iscsiPortal, iscsiIQN, iscsiLUN,
		iscsiFSType)
	if iscsiID != duplicateISCSI {
		t.Errorf("Duplicate iscsi not detected.  Expected %d; got %d", iscsiID,
			duplicateISCSI)
	}

	iscsiID2 := manager.InsertISCSI(iscsiPortal2, iscsiIQN2, iscsiLUN2,
		iscsiFSType)
	if iscsiID2 == iscsiID {
		t.Error("Duplicate ISCSI ID returned for new ISCSI backend; got ",
			iscsiID2)
	}
	correct = tu.ValidateResult(t,
		fmt.Sprintf("SELECT target_portal, iqn, lun, fs_type FROM iscsi "+
			"WHERE id = %d", iscsiID2),
		[]interface{}{iscsiPortal2, iscsiIQN2, iscsiLUN2, iscsiFSType2},
		[]reflect.Type{tu.StringType, tu.StringType, tu.IntType, tu.StringType},
	)

	pv_time := unversioned.Now()
	manager.InsertPV(pv_uid, pv_name, pv_time, iscsiID, dbmanager.ISCSI,
		pv_storage, pv_access_modes, pv_json, strconv.Itoa(rv))
	correct = tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			" nfs_id, iscsi_id, storage, access_modes, json FROM pv WHERE uid "+
			"LIKE '"+pv_uid+"'",
		[]interface{}{pv_uid, pv_name, pv_time.Time, nil, 0, iscsiID,
			pv_storage, GetAccessModeString(pv_access_modes), pv_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.IntType, tu.IntType, tu.Int64Type, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Unable to create DB entry for ISCSI PV")
	}
}
