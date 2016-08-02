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
	"log"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api"

	tu "github.com/netapp/kubevoltracker/dbmanager/mysql/testutils"
	"github.com/netapp/kubevoltracker/resources"
)
import _ "github.com/go-sql-driver/mysql"

var manager *mySQLManager

const (
	testDB = "kubevoltracker_test"

	test_ns     = "test-namespace-db"
	test_ns_alt = "test-namespace-db-alt"

	watcher_ns     = "test-watcher-namespace-db"
	watcher_ns_alt = "test-watcher-namespace-db-alt"

	nfs_server = "127.0.0.1"
	nfs_path   = "test-path"

	nfs_server_2 = "127.0.0.2"
	nfs_path_2   = "test-path2"

	iscsiPortal = "127.0.0.1"
	iscsiIQN    = "iqn.2016-05.com.netapp:storage:example-iqn"
	iscsiLUN    = 0
	iscsiFSType = "ext4"

	iscsiPortal2 = "127.0.0.2"
	iscsiIQN2    = "iqn.2016-05.com.netapp:storage:other-example"
	iscsiLUN2    = 1
	iscsiFSType2 = "ext4"

	pv_uid     = "test-pv-001"
	pv_name    = "test-volume"
	pv_storage = int64(400)
	pv_json    = "Insert PV JSON here."

	pvc_uid     = "test-pvc-001"
	pvc_name    = "test-pvc"
	pvc_storage = int64(200)
	pvc_json    = "Insert PVC JSON here."

	pod_uid  = "test-pod-001"
	pod_name = "test-pod"
	pod_json = "Insert pod JSON here."

	vol_pod_uid  = "test-vol-pod-001"
	vol_pod_name = "test-vol-pod"
	vol_pod_json = "Insert pod with volume mount JSON here."

	readOnlyDefault = false

	// TestMatchPodMount
	pod_mount_pvc_name = "test-pvc-pod-mount"

	pvc_old_uid  = "test-pvc-002"
	pvc_old_json = "Insert old PVC JSON here."

	pvc_current_uid  = "test-pvc-003"
	pvc_current_json = "Insert current PVC JSON here."

	pvc_future_uid  = "test-pvc-004"
	pvc_future_json = "Insert future PVC JSON here."

	pvc_other_uid                 = "test-pvc-005"
	pod_mount_pvc_other_name      = "test-pvc-pod-mount-other"
	pvc_other_json                = "Insert other PVC JSON here."
	pvc_concurrent_uid            = "test-pvc-006"
	pod_mount_pvc_concurrent_name = "test-pvc-pod-mount-concurrent"
	pvc_concurrent_json           = "Insert other PVC JSON here."

	pod_mount_uid  = "test-vol-pod-002"
	pod_mount_name = "test-match-pod-mount"
	pod_mount_json = "Insert pod for mount matching test JSON here."

	pod_mount_future_uid  = "test-vol-pod-003"
	pod_mount_future_name = "test-match-pod-mount-future"
	pod_mount_future_json = "Insert JSON for non-matching pod here."

	pv_update_storage = int64(1000)
	pv_update_json    = "Updated PV JSON"

	pvc_update_storage = int64(900)
	pvc_update_json    = "Updated PVC JSON"
)

var (
	pv_access_modes = []api.PersistentVolumeAccessMode{api.ReadWriteOnce,
		api.ReadOnlyMany}
	pvc_access_modes        = []api.PersistentVolumeAccessMode{api.ReadWriteOnce}
	pv_update_access_modes  = []api.PersistentVolumeAccessMode{api.ReadOnlyMany}
	pvc_update_access_modes = []api.PersistentVolumeAccessMode{
		api.ReadWriteMany, api.ReadOnlyMany}

	container1 = resources.ContainerDesc{
		Name:      "test-container1",
		Image:     "test-program",
		Command:   "/bin/sh",
		PVCMounts: []resources.VolumeMount{},
	}

	container2 = resources.ContainerDesc{
		Name:      "test-container2",
		Image:     "test-binary",
		Command:   "/bin/bash",
		PVCMounts: []resources.VolumeMount{},
	}

	volContainer1 = resources.ContainerDesc{
		Name:    "volContainer1",
		Image:   "test-program",
		Command: "/bin/sh",
		PVCMounts: []resources.VolumeMount{
			resources.VolumeMount{Name: pvc_name, ReadOnly: readOnlyDefault},
		},
	}

	podMountContainer = resources.ContainerDesc{
		Name:    "podMountContainer",
		Image:   "test-program",
		Command: "/bin/sh",
		PVCMounts: []resources.VolumeMount{
			resources.VolumeMount{Name: pod_mount_pvc_name,
				ReadOnly: readOnlyDefault},
		},
	}

	multiMountContainer = resources.ContainerDesc{
		Name:    "podMountContainer",
		Image:   "test-program",
		Command: "/bin/sh",
		PVCMounts: []resources.VolumeMount{
			resources.VolumeMount{Name: pod_mount_pvc_name,
				ReadOnly: readOnlyDefault},
			resources.VolumeMount{Name: pod_mount_pvc_other_name,
				ReadOnly: readOnlyDefault},
		},
	}
)

func (manager *mySQLManager) clearTestTables() {
	_, err := manager.db.Exec("DELETE FROM pod WHERE uid LIKE 'test-%';")
	if err != nil {
		log.Fatal("Unable to delete test pods: ", err)
	}
	_, err = manager.db.Exec("DELETE FROM container WHERE pod_uid LIKE " +
		"'test-%';")
	if err != nil {
		log.Fatal("Unable to delete test containers: ", err)
	}
	_, err = manager.db.Exec("DELETE FROM pv WHERE uid LIKE 'test-%';")
	if err != nil {
		log.Fatal("Unable to delete test pvs: ", err)
	}
	_, err = manager.db.Exec("DELETE FROM pvc WHERE uid LIKE 'test-%';")
	if err != nil {
		log.Fatal("Unable to delete test pvcs: ", err)
	}
	_, err = manager.db.Exec("DELETE FROM pod_mount WHERE pod_uid LIKE " +
		"'test-%';")
	if err != nil {
		log.Fatal("Unable to delete test pvcs: ", err)
	}
	_, err = manager.db.Exec("DELETE FROM nfs WHERE ip_addr = inet_aton('" +
		nfs_server + "') and (path LIKE '" + nfs_path + "' or path LIKE " +
		"'/path')")
	if err != nil {
		log.Fatal("Unable to delete test NFS volume sources: ", err)
	}
	_, err = manager.db.Exec("DELETE FROM resource_version WHERE namespace "+
		"LIKE ? OR namespace LIKE ? OR namespace LIKE ?", watcher_ns,
		watcher_ns_alt, resources.PVNamespace)
	if err != nil {
		log.Fatal("Unable to delete resource versions: ", err)
	}
}

func TestMain(m *testing.M) {
	manager = NewParams("root", "root", os.Getenv("MYSQL_IP"), testDB,
		"parseTime=true").(*mySQLManager)
	defer manager.Destroy()
	tu.InitDB(testDB)
	defer tu.DestroyDB()
	os.Exit(m.Run())
}
