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

// Tests the case where the PV and the PVC are created in the standard order.
func TestBasicBind(t *testing.T) {
	manager.clearTestTables()
	rv := 700
	// We need this so we can meet foreign key constraints.
	nfs_id := manager.InsertNFS(nfs_server, nfs_path)

	if nfs_id <= 0 {
		t.Error("Invalid ID received during insert:  ", nfs_id)
	}

	pv_time := unversioned.Now()
	manager.InsertPV(pv_uid, pv_name, pv_time, nfs_id, dbmanager.NFS,
		pv_storage, pv_access_modes, pv_json, strconv.Itoa(rv))
	rv++

	pvc_time := unversioned.Now()
	manager.InsertPVC(pvc_uid, pvc_name, pvc_time, test_ns_alt, pvc_storage,
		pvc_access_modes, pvc_json, watcher_ns_alt, strconv.Itoa(rv))
	rv++

	bind_time := unversioned.Now()
	manager.BindPVC(pv_uid, pvc_uid, bind_time, strconv.Itoa(rv))

	correct := tu.ValidateResult(t, "SELECT uid, name, create_time,"+
		" bind_time, delete_time, pv_uid, namespace, json FROM pvc WHERE uid "+
		"like '"+pvc_uid+"'",
		[]interface{}{pvc_uid, pvc_name, pvc_time.Time, bind_time.Time, nil,
			pv_uid, test_ns_alt, pvc_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.TimeType, tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Basic bind failed.")
	} else {
		t.Log("Basic bind succeeded.")
	}
	tu.ValidateResourceVersion(t, resources.PVs, resources.PVNamespace, strconv.Itoa(rv))
}

func TestOutOfOrderBind(t *testing.T) {
	manager.clearTestTables()
	// We need this so we can meet foreign key constraints.
	nfs_id := manager.InsertNFS(nfs_server, nfs_path)

	if nfs_id <= 0 {
		t.Error("Invalid ID received during insert:  ", nfs_id)
	}

	pv_time := unversioned.Now()
	pvc_time := unversioned.Now()
	manager.InsertPV(pv_uid, pv_name, pv_time, nfs_id, dbmanager.NFS,
		pv_storage, pv_access_modes, pv_json, "400")

	bind_time := unversioned.Now()
	manager.BindPVC(pv_uid, pvc_uid, bind_time, "401")
	tu.ValidateResourceVersion(t, resources.PVs, resources.PVNamespace, "401")

	correct := tu.ValidateResult(t, "SELECT uid, create_time, bind_time, "+
		"delete_time, pv_uid from pvc WHERE uid like '"+pvc_uid+"'",
		[]interface{}{pvc_uid, nil, bind_time.Time, nil, pv_uid},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.TimeType, tu.TimeType,
			tu.StringType},
	)
	if !correct {
		t.Error("Partial bind incorrect.")
	} else {
		t.Log("Executed partial bind correctly.")
	}

	manager.InsertPVC(pvc_uid, pvc_name, pvc_time, test_ns_alt, pvc_storage,
		pvc_access_modes, pvc_json, watcher_ns_alt, "200")
	correct = tu.ValidateResult(t, "SELECT uid, name, create_time,"+
		" bind_time, delete_time, pv_uid, namespace, json FROM pvc WHERE uid "+
		"like '"+pvc_uid+"'",
		[]interface{}{pvc_uid, pvc_name, pvc_time.Time, bind_time.Time, nil,
			pv_uid, test_ns_alt, pvc_json},
		[]reflect.Type{tu.StringType, tu.StringType, tu.TimeType, tu.TimeType,
			tu.TimeType, tu.StringType, tu.StringType, tu.StringType},
	)
	if !correct {
		t.Error("Out of order bind improperly finalized.")
	} else {
		t.Log("Out of order bind finalized correctly.")
	}
	tu.ValidateResourceVersion(t, resources.PVs, resources.PVNamespace, "401")
	tu.ValidateResourceVersion(t, resources.PVCs, watcher_ns_alt, "200")
}
