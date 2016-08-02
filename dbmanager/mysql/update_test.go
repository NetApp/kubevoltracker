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
	"strconv"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/netapp/kubevoltracker/dbmanager"
	tu "github.com/netapp/kubevoltracker/dbmanager/mysql/testutils"
	"github.com/netapp/kubevoltracker/resources"
)

func TestUpdatePV(t *testing.T) {
	rv := 1000

	manager.clearTestTables()

	nfs_id := manager.InsertNFS(nfs_server, nfs_path)
	if nfs_id <= 0 {
		t.Fatal("Invalid initial NFS ID for update:  ", nfs_id)
	}

	nfs_id_2 := manager.InsertNFS(nfs_server_2, nfs_path_2)
	if nfs_id_2 <= 0 {
		t.Fatal("Invalid updated NFS ID for update:  ", nfs_id_2)
	}
	if nfs_id == nfs_id_2 {
		t.Fatal("NFS IDs are equal; this should not happen.")
	}

	pvTime := unversioned.Now()
	manager.InsertPV(pv_uid, pv_name, pvTime, nfs_id, dbmanager.NFS,
		pv_storage, pv_access_modes, pv_json, strconv.Itoa(rv))
	rv++
	// Assume that this works, since we check it in TestInsert
	manager.UpdatePV(pv_uid, nfs_id_2, dbmanager.NFS, pv_update_storage,
		pv_update_access_modes, pv_update_json, strconv.Itoa(rv))
	if correct := tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			" nfs_id, iscsi_id, storage, access_modes, json FROM pv WHERE uid "+
			"LIKE '"+pv_uid+"'",
		[]interface{}{pv_uid, pv_name, pvTime.Time, nil, nfs_id_2, 0,
			pv_update_storage, GetAccessModeString(pv_update_access_modes),
			pv_update_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.IntType, tu.IntType, tu.Int64Type, tu.StringType, tu.StringType},
	); !correct {
		t.Errorf("PV update failed.")
	}
	tu.ValidateResourceVersion(t, resources.PVs, resources.PVNamespace,
		strconv.Itoa(rv))
}

func TestUpdateISCSIPV(t *testing.T) {
	rv := 1004

	manager.clearTestTables()

	iscsi1 := manager.InsertISCSI(iscsiPortal, iscsiIQN, iscsiLUN, iscsiFSType)
	if iscsi1 < 0 {
		t.Fatal("Invalid initial ISCSI ID:  ", iscsi1)
	}

	iscsi2 := manager.InsertISCSI(iscsiPortal2, iscsiIQN2, iscsiLUN2,
		iscsiFSType)
	if iscsi2 < 0 {
		t.Fatal("Invalid update ISCSI ID:  ", iscsi1)
	}

	pvTime := unversioned.Now()
	manager.InsertPV(pv_uid, pv_name, pvTime, iscsi1, dbmanager.ISCSI,
		pv_storage, pv_access_modes, pv_json, strconv.Itoa(rv))
	rv++
	// Note that these access modes probably aren't possible for ISCSI,
	// but the database should be agnostic about this.
	manager.UpdatePV(pv_uid, iscsi2, dbmanager.ISCSI, pv_update_storage,
		pv_update_access_modes, pv_update_json, strconv.Itoa(rv))
	if correct := tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			" nfs_id, iscsi_id, storage, access_modes, json FROM pv WHERE uid "+
			"LIKE '"+pv_uid+"'",
		[]interface{}{pv_uid, pv_name, pvTime.Time, nil, 0, iscsi2,
			pv_update_storage, GetAccessModeString(pv_update_access_modes),
			pv_update_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.IntType, tu.IntType, tu.Int64Type, tu.StringType, tu.StringType},
	); !correct {
		t.Errorf("PV update failed.")
	}
}

func TestUpdatePVC(t *testing.T) {
	rv := 9713
	manager.clearTestTables()

	pvc_time := unversioned.Now()
	manager.InsertPVC(pvc_uid, pvc_name, pvc_time, test_ns, pvc_storage,
		pvc_access_modes, pvc_json, watcher_ns, strconv.Itoa(rv))
	// Assume that this works, since we check it in TestInsert
	rv++
	manager.UpdatePVC(pvc_uid, pvc_update_storage, pvc_update_access_modes,
		pvc_update_json, watcher_ns, strconv.Itoa(rv))
	if correct := tu.ValidateResult(t,
		"SELECT uid, name, create_time, delete_time,"+
			" pv_uid, namespace, storage, access_modes, json FROM pvc WHERE "+
			"uid LIKE '"+pvc_uid+"'",
		[]interface{}{pvc_uid, pvc_name, pvc_time.Time, nil, "", test_ns,
			pvc_update_storage, GetAccessModeString(pvc_update_access_modes),
			pvc_update_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.StringType, tu.StringType, tu.Int64Type, tu.StringType,
			tu.StringType},
	); !correct {
		t.Error("PVC failed to update.")
	}
	tu.ValidateResourceVersion(t, resources.PVCs, watcher_ns, strconv.Itoa(rv))
}
