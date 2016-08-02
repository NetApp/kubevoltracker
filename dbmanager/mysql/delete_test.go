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
	"reflect"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/netapp/kubevoltracker/dbmanager"
	tu "github.com/netapp/kubevoltracker/dbmanager/mysql/testutils"
	"github.com/netapp/kubevoltracker/resources"
)

func TestDeletePod(t *testing.T) {
	manager.clearTestTables()
	var insertRV = "10"
	var deleteRV = "11"

	pod_time := unversioned.Now()
	delete_time := unversioned.NewTime(pod_time.Add(time.Second))
	manager.InsertPod(pod_uid, pod_name, pod_time, test_ns, nil, pod_json,
		watcher_ns, insertRV)
	manager.DeletePod(pod_uid, delete_time, watcher_ns, deleteRV)
	correct := tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			"namespace, json FROM pod WHERE uid LIKE '"+pod_uid+"'",
		[]interface{}{pod_uid, pod_name, pod_time.Time, delete_time.Time,
			test_ns, pod_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Pod not updated with delete info correctly.")
	} else {
		t.Log("Delete correctly registered for pod.")
	}
	tu.ValidateResourceVersion(t, resources.Pods, watcher_ns, deleteRV)
}

func TestDeletePVC(t *testing.T) {
	manager.clearTestTables()
	var insertRV = "20"
	var deleteRV = "21"

	pvcTime := unversioned.Now()
	deleteTime := unversioned.NewTime(pvcTime.Add(time.Second))
	manager.InsertPVC(pvc_uid, pvc_name, pvcTime, test_ns, pvc_storage,
		pvc_access_modes, pvc_json, watcher_ns, insertRV)
	manager.DeletePVC(pvc_uid, deleteTime, watcher_ns, deleteRV)
	correct := tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			"bind_time, pv_uid, namespace, json FROM pvc WHERE uid LIKE '"+
			pvc_uid+"'",
		[]interface{}{pvc_uid, pvc_name, pvcTime.Time, deleteTime.Time, nil, "",
			test_ns, pvc_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.TimeType, tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("PVC not updated with delete info correctly.")
	} else {
		t.Log("Delete correctly registered for PVC.")
	}
	tu.ValidateResourceVersion(t, resources.PVCs, watcher_ns, deleteRV)
}

func TestDeletePV(t *testing.T) {
	manager.clearTestTables()
	var insertRV = "30"
	var deleteRV = "31"

	// Inserted to meet foreign key dependencies, even though they're not strict
	nfsID := manager.InsertNFS(nfs_server, nfs_path)
	if nfsID <= 0 {
		t.Error("Invalid ID received during insert:  ", nfsID)
	}

	pvTime := unversioned.Now()
	deleteTime := unversioned.NewTime(pvTime.Add(time.Second))
	manager.InsertPV(pv_uid, pv_name, pvTime, nfsID, dbmanager.NFS, pv_storage,
		pv_access_modes, pv_json, insertRV)
	manager.DeletePV(pv_uid, deleteTime, deleteRV)
	correct := tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			" nfs_id, json FROM pv WHERE uid like '"+pv_uid+"'",
		[]interface{}{pv_uid, pv_name, pvTime.Time, deleteTime.Time, nfsID,
			pv_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.IntType, tu.StringType},
	)
	if !correct {
		t.Error("Unable to update PV with delete time.")
	} else {
		t.Log("Successfully updated PV with delete time.")
	}
	tu.ValidateResourceVersion(t, resources.PVs, resources.PVNamespace,
		deleteRV)
}

func TestFixPodMount(t *testing.T) {
	manager.clearTestTables()
	pod_time := unversioned.Now()
	pod_delete_time := unversioned.NewTime(pod_time.Add(time.Second * 1))
	pvc_current_time := unversioned.NewTime(pod_time.Add(time.Second * 2))

	pod_future_time := unversioned.NewTime(pod_time.Add(time.Second * 5))
	pod_future_delete := unversioned.NewTime(pod_time.Add(time.Second * 6))

	// Insert a pod that matches the spec
	manager.InsertPVC(pvc_other_uid, pod_mount_pvc_other_name, pod_time,
		test_ns_alt, pvc_storage, pvc_access_modes, pvc_other_json,
		watcher_ns, "6")
	// Insert a pod that does not match the spec
	manager.InsertPVC(pvc_current_uid, pod_mount_pvc_name, pvc_current_time,
		test_ns_alt, pvc_storage, pvc_access_modes, pvc_current_json,
		watcher_ns, "7")
	// This next insert adds another pvc that creates after the deletion
	// (tests a failure case with the original code)
	manager.InsertPVC(pvc_concurrent_uid, pod_mount_pvc_concurrent_name,
		pvc_current_time,
		test_ns_alt, pvc_storage, pvc_access_modes, pvc_concurrent_json,
		watcher_ns, "7")
	manager.InsertPod(pod_mount_uid, pod_mount_name, pod_time, test_ns,
		[]resources.ContainerDesc{multiMountContainer}, pod_mount_json,
		watcher_ns, "8")
	manager.DeletePod(pod_mount_uid, pod_delete_time, watcher_ns, "9")
	if correct := tu.ValidateResult(t,
		"SELECT count(*) FROM pod_mount WHERE pod_uid LIKE '"+pod_mount_uid+
			"'",
		[]interface{}{0},
		[]reflect.Type{tu.IntType},
	); !correct {
		t.Error("Pod deletion failed to clean up erroneous pod mount match.")
	}

	manager.InsertPod(pod_mount_future_uid, pod_mount_future_name,
		pod_future_time, test_ns_alt,
		[]resources.ContainerDesc{multiMountContainer},
		pod_mount_future_json, watcher_ns, "10")
	manager.DeletePod(pod_mount_future_uid, pod_future_delete, watcher_ns,
		"11")
	if correct := tu.ValidateResult(t,
		"SELECT pod_uid, pvc_uid, pvc_name FROM "+
			"pod_mount WHERE pod_uid LIKE '"+pod_mount_future_uid+"' AND "+
			"pvc_uid LIKE '"+pvc_current_uid+"'",
		[]interface{}{pod_mount_future_uid, pvc_current_uid,
			pod_mount_pvc_name},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType},
	); !correct {
		t.Error("Pod deletion erroneously cleared correct pod mount match.")
	}
	if correct := tu.ValidateResult(t,
		"SELECT pod_uid, pvc_uid, pvc_name FROM "+
			"pod_mount WHERE pod_uid LIKE '"+pod_mount_future_uid+"' AND "+
			"pvc_uid LIKE '"+pvc_other_uid+"'",
		[]interface{}{pod_mount_future_uid, pvc_other_uid,
			pod_mount_pvc_other_name},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType},
	); !correct {
		t.Error("Pod deletion erroneously cleared correct pod mount match.")
	}
}
