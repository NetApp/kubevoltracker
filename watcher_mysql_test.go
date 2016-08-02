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
	"database/sql"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	gomysql "github.com/go-sql-driver/mysql"

	"k8s.io/kubernetes/pkg/api"

	"github.com/netapp/kubevoltracker/dbmanager"
	"github.com/netapp/kubevoltracker/dbmanager/mysql"
	tu "github.com/netapp/kubevoltracker/dbmanager/mysql/testutils"
	"github.com/netapp/kubevoltracker/kubectl"
	"github.com/netapp/kubevoltracker/resources"
)

const testDB = "kubevoltracker_test"

var (
	defaultPVAccessModes = []api.PersistentVolumeAccessMode{api.ReadWriteMany,
		api.ReadWriteOnce, api.ReadOnlyMany}
	defaultPVCAccessModes = []api.PersistentVolumeAccessMode{api.ReadWriteOnce,
		api.ReadWriteMany}
)

var tableForResource = map[resources.ResourceType]dbmanager.Table{
	resources.Pods: dbmanager.Pod,
	resources.PVCs: dbmanager.PVC,
	resources.PVs:  dbmanager.PV,
}

type resourceAttrs struct {
	uid        string
	createTime time.Time
	rv         string
	fileName   string
}

func getTableCount(tableName string) int {
	var count int
	err := tu.DB.QueryRow("SELECT COUNT(*) FROM " + tableName).Scan(&count)
	if err != nil {
		log.Fatalf("Unable to query table %s:  %v\n", tableName, err)
	}
	return count
}

func GetStartingCounts() map[dbmanager.Table]int {
	ret := make(map[dbmanager.Table]int)
	var tableTypes = []dbmanager.Table{dbmanager.Pod, dbmanager.PVC,
		dbmanager.PV, dbmanager.NFS, dbmanager.PodMount}
	for _, tableName := range tableTypes {
		ret[tableName] = getTableCount(string(tableName))
	}
	return ret
}

// This is necessary because we may have missed events when running mock
// tests of the other functions; once we've started the watchers, we'll need
// to make sure they've processed everything before we do anything.
func QuiesceTableCounts(counts map[dbmanager.Table]int) {
	start := time.Now()
	for changing := true; changing; {
		time.Sleep(time.Second)
		changing = false
		for table, oldCount := range counts {
			newCount := getTableCount(string(table))
			if newCount != oldCount {
				changing = true
				counts[table] = newCount
			}
		}
	}
	end := time.Now()
	log.Printf("Tables took %s to quiesce", end.Sub(start))
}

func GetMySQLWatcherForNamespace(namespace string) *Watcher {
	manager := mysql.NewForDB("root", "root", os.Getenv("MYSQL_IP"), testDB)
	w, err := NewWatcher(namespace, os.Getenv("KUBERNETES_MASTER"), manager)
	if err != nil {
		log.Fatal("Unable to instantiate watcher; aborting: ", err)
	}
	return w
}

func GetMySQLWatcher() *Watcher {
	return GetMySQLWatcherForNamespace(watcherNS)
}

func CheckCount(t *testing.T, table dbmanager.Table, tableDesc string,
	expected int) {
	newCount := getTableCount(string(table))
	if newCount != expected {
		t.Errorf("Got wrong count for %s; expected %d, got %d\n", table,
			expected, newCount)
	}
}

func ValidateContainers(t *testing.T, podUID string,
	containers []resources.ContainerDesc, name string) {

	correct := tu.ValidateResult(t,
		"SELECT COUNT(*) FROM container WHERE pod_uid LIKE '"+podUID+"'",
		[]interface{}{len(containers)},
		[]reflect.Type{tu.IntType},
	)
	if !correct {
		t.Errorf("Incorrect number of containers for %s.\n", name)
	}

	for i, container := range containers {
		correct := tu.ValidateResult(t,
			"SELECT image, command FROM container WHERE pod_uid LIKE '"+
				podUID+"' and name LIKE '"+container.Name+"'",
			[]interface{}{container.Image, container.Command},
			[]reflect.Type{tu.StringType, tu.StringType},
		)
		if !correct {
			t.Errorf("Container %d incorrect for %s\n", i, name)
		}
	}
}

func ValidateOperationTime(t *testing.T, table dbmanager.Table,
	operation string, colName string, uid string, createTime time.Time) {

	var opTime gomysql.NullTime
	err := tu.DB.QueryRow("SELECT " + colName + " FROM " + string(table) +
		" WHERE uid LIKE '" + string(uid) + "'").Scan(&opTime)
	if err != nil {
		t.Error("Unable to query ", table, " for ", operation, " time: ", err)
	}
	if !opTime.Valid {
		t.Error("Invalid", operation, "time found for", table,
			".  Expected not null.")
	} else if opTime.Time.Before(createTime) ||
		opTime.Time.Equal(createTime) {
		t.Errorf("Invalid %s time:  before or equal to create time.\n\t"+
			"%s time: %v\n\tCreate time: %v\n\t", operation, operation,
			opTime.Time, createTime)
	}
}

func ValidateChangedRV(t *testing.T, resource resources.ResourceType,
	namespace string, prevRV string) string {

	var newRV string

	err := tu.DB.QueryRow("SELECT resource_version FROM resource_version "+
		"WHERE resource LIKE ? and namespace like ?", string(resource),
		namespace).Scan(&newRV)
	if err != nil {
		t.Fatalf("Unable to get resource version for %s, namespace %s:  %s\n",
			resource, namespace, err)
	}
	if prevRV != "" && newRV == "" {
		t.Errorf("Resource version erased for resource %s, namespace %s\n",
			resource, namespace)
	}
	if prevRV == newRV {
		t.Errorf("RV for resource %s, namespace %s not updated; got %s\n",
			resource, namespace, newRV)
	}
	return newRV
}

func GetRV(resource resources.ResourceType, namespace string) string {
	var rv string

	err := tu.DB.QueryRow("SELECT resource_version FROM resource_version "+
		"WHERE resource LIKE ? AND namespace LIKE ?", string(resource),
		namespace).Scan(&rv)
	if err == sql.ErrNoRows {
		return ""
	} else if err != nil {
		log.Panic("Get RV query failed: ", err)
	}
	return rv
}

func GetMostRecentUIDTime(t *testing.T, name string, r resources.ResourceType) (
	uid string, createTime time.Time) {

	var err error = nil
	// Try to get the most recent UID for five seconds.
	for tries := 0; (err == nil || err == sql.ErrNoRows) && tries < 10; tries++ {
		time.Sleep(500 * time.Millisecond)
		uidQuery := fmt.Sprintf("SELECT uid, create_time FROM %s WHERE "+
			"create_time = (SELECT max(create_time) FROM %s) and NAME LIKE "+
			"'%s'", tableForResource[r], tableForResource[r], name)
		err = tu.DB.QueryRow(uidQuery).Scan(&uid, &createTime)
	}
	if err != nil {
		t.Fatalf("Unable to query DB for most recent resource UID for %s:  %s",
			name, err)
	}
	return uid, createTime
}

func CreateResource(t *testing.T,
	resource resources.ResourceType,
	name string,
	resourceConstructor func() (string, error),
	queryConstructor func(resourceAttrs) string,
	expectedVals []interface{},
	valTypes []reflect.Type,
	expectedCount int,
	resourceDesc string) resourceAttrs {

	var rvNamespace, fileName string
	var ret resourceAttrs
	var err error

	if resource == resources.PVs {
		rvNamespace = resources.PVNamespace
	} else {
		rvNamespace = watcherNS
	}
	origRV := GetRV(resource, rvNamespace)
	startTime := time.Now().Truncate(time.Second)
	if fileName, err = resourceConstructor(); err != nil {
		t.Fatalf("Unable to create %s: %s", fileName, err)
	}
	ret.fileName = fileName
	ret.uid, ret.createTime = GetMostRecentUIDTime(t, name, resource)
	if ret.createTime.Before(startTime) {
		t.Errorf("%s time error:\n\tDB create time: %s\n\tLocal time:  %s",
			resourceDesc, ret.createTime, startTime)
	}
	CheckCount(t, tableForResource[resource], resourceDesc, expectedCount)
	correct := tu.ValidateResult(t, queryConstructor(ret), expectedVals,
		valTypes)
	if !correct {
		t.Errorf("Incorrect value(s) found in database for %s with UID %s.",
			resourceDesc, ret.uid)
	}
	ret.rv = ValidateChangedRV(t, resource, rvNamespace, origRV)
	return ret
}

func TestDBWatches(t *testing.T) {
	var pvAttrs, pvcAttrs, podAttrs, noVolPodAttrs resourceAttrs
	var (
		pvName        = "nfs"
		pvcName       = "nfs"
		podName       = "vol-test"
		noVolPodName  = "no-vol-test"
		readOnlyMount = false

		size = 1

		podContainers = kubectl.GetContainerDescs(
			[][]resources.VolumeMount{
				[]resources.VolumeMount{
					resources.VolumeMount{
						Name: pvcName, ReadOnly: readOnlyMount,
					},
				},
			},
		)
		noVolPodContainers = kubectl.GetContainerDescs(
			[][]resources.VolumeMount{[]resources.VolumeMount{}})
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	countMap := GetStartingCounts()
	watcher.Watch(resources.PVs, false)
	watcher.Watch(resources.PVCs, false)
	watcher.Watch(resources.Pods, false)
	QuiesceTableCounts(countMap)

	pvAttrs = CreateResource(t, resources.PVs, pvName,
		func() (string, error) {
			return kubectl.CreatePV(pvName, size, defaultPVAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT p.name, p.delete_time, "+
				"p.storage, p.access_modes, p.iscsi_id, inet_ntoa(n.ip_addr), "+
				"n.path FROM pv p, nfs n WHERE p.nfs_id = n.id AND p.uid "+
				"LIKE '%s'", r.uid)
		},
		[]interface{}{pvName, nil, kubectl.GetStorageValue(size),
			mysql.GetAccessModeString(defaultPVAccessModes), 0,
			kubectl.GetFilerIP(), kubectl.GetExportRoot()},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.Int64Type,
			tu.StringType, tu.IntType, tu.StringType, tu.StringType},
		countMap[dbmanager.PV]+1, "PV")

	pvcAttrs = CreateResource(t, resources.PVCs, pvcName,
		func() (string, error) {
			return kubectl.CreatePVC(pvcName, watcherNS, size,
				defaultPVCAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT name, delete_time, "+
				"namespace, storage, access_modes, pv_uid FROM pvc WHERE uid "+
				"LIKE '%s'", r.uid)
		},
		[]interface{}{pvcName, nil, watcherNS, size * 1024 * 1024,
			mysql.GetAccessModeString(defaultPVCAccessModes),
			string(pvAttrs.uid)},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.StringType, tu.IntType,
			tu.StringType, tu.StringType},
		countMap[dbmanager.PVC]+1, "PVC")
	ValidateOperationTime(t, dbmanager.PVC, "bind", "bind_time",
		pvcAttrs.uid, pvcAttrs.createTime)

	podAttrs = CreateResource(t, resources.Pods, podName,
		func() (string, error) {
			return kubectl.CreatePod(podName, watcherNS, podContainers)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT name, delete_time, "+
				"namespace FROM pod WHERE uid LIKE '%s'", r.uid)
		},
		[]interface{}{podName, nil, watcherNS},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.StringType},
		countMap[dbmanager.Pod]+1, "NFS pod")
	CheckCount(t, dbmanager.PodMount, "pod_mount",
		countMap[dbmanager.PodMount]+1)
	correct := tu.ValidateResult(t,
		"SELECT pod_uid, pvc_uid, container_name, pvc_name, read_only "+
			"FROM pod_mount WHERE pod_uid LIKE '"+podAttrs.uid+"'",
		[]interface{}{podAttrs.uid, pvcAttrs.uid, podContainers[0].Name,
			pvcName, readOnlyMount},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType,
			tu.StringType, tu.BoolType},
	)
	if !correct {
		t.Error("Incorrect entry found in pod_mount table")
	}
	ValidateContainers(t, podAttrs.uid, podContainers, podName)

	noVolPodAttrs = CreateResource(t, resources.Pods, noVolPodName,
		func() (string, error) {
			return kubectl.CreatePod(noVolPodName, watcherNS,
				noVolPodContainers)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT name, delete_time, "+
				"namespace FROM pod WHERE uid LIKE '%s'", r.uid)
		},
		[]interface{}{noVolPodName, nil, watcherNS},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.StringType},
		countMap[dbmanager.Pod]+2, "pod without volumes")
	CheckCount(t, dbmanager.PodMount, "pod_mount after creating pod without"+
		" volumes", countMap[dbmanager.PodMount]+1)

	correct = tu.ValidateResult(t,
		"SELECT COUNT(*) FROM pod_mount WHERE "+
			"pod_uid LIKE '"+noVolPodAttrs.uid+"'",
		[]interface{}{0},
		[]reflect.Type{tu.IntType},
	)
	if !correct {
		t.Error("Found no volume pod uid in pod_mount table.")
	}
	ValidateContainers(t, noVolPodAttrs.uid, noVolPodContainers, noVolPodName)

	for _, name := range []string{podAttrs.fileName, pvcAttrs.fileName,
		pvAttrs.fileName} {

		if err := kubectl.RunKubectl(kubectl.Delete, name, true); err != nil {
			t.Fatalf("Unable to delete %s:  %s", name, err)
		}
	}
	time.Sleep(time.Second)
	ValidateOperationTime(t, dbmanager.Pod, "delete", "delete_time",
		podAttrs.uid, podAttrs.createTime)
	ValidateOperationTime(t, dbmanager.PVC, "delete", "delete_time",
		pvcAttrs.uid, pvcAttrs.createTime)
	ValidateOperationTime(t, dbmanager.PV, "delete", "delete_time",
		pvAttrs.uid, pvAttrs.createTime)
	ValidateChangedRV(t, resources.Pods, watcherNS, noVolPodAttrs.rv)
	ValidateChangedRV(t, resources.PVCs, watcherNS, pvcAttrs.rv)
	ValidateChangedRV(t, resources.PVs, resources.PVNamespace, pvAttrs.rv)
}

func TestReadWriteMounts(t *testing.T) {
	var (
		// TODO:  This is probably excessive.
		accessModes = [][]api.PersistentVolumeAccessMode{
			[]api.PersistentVolumeAccessMode{api.ReadWriteOnce},
			[]api.PersistentVolumeAccessMode{api.ReadOnlyMany},
			[]api.PersistentVolumeAccessMode{api.ReadWriteMany},
			[]api.PersistentVolumeAccessMode{api.ReadWriteOnce,
				api.ReadWriteMany},
			[]api.PersistentVolumeAccessMode{api.ReadWriteOnce,
				api.ReadOnlyMany},
			[]api.PersistentVolumeAccessMode{api.ReadOnlyMany,
				api.ReadWriteMany},
			[]api.PersistentVolumeAccessMode{api.ReadWriteOnce,
				api.ReadOnlyMany, api.ReadWriteMany},
		}
	)
	type volumeDescription struct {
		name        string
		size        int
		accessModes []api.PersistentVolumeAccessMode
	}
	volDescs := make([]volumeDescription, len(accessModes))
	// For now, have one volume per container.
	volMounts := make([][]resources.VolumeMount, len(accessModes))
	// Create a PV, PVC, and container-mounted volume for each access mode.
	for i, modes := range accessModes {
		volDescs[i] = volumeDescription{
			name:        fmt.Sprintf("nfs-%d", i),
			size:        i + 1,
			accessModes: modes,
		}
		readOnly := false
		for _, mode := range modes {
			if mode == api.ReadOnlyMany {
				readOnly = true
			}
		}
		volMounts[i] = []resources.VolumeMount{
			resources.VolumeMount{
				Name: fmt.Sprintf("nfs-%d", i), ReadOnly: readOnly,
			},
		}
	}
	containerDescs := kubectl.GetContainerDescs(volMounts)
	pvcAttrs := make([]resourceAttrs, len(volDescs))

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	countMap := GetStartingCounts()
	watcher.Watch(resources.PVs, false)
	watcher.Watch(resources.PVCs, false)
	watcher.Watch(resources.Pods, false)
	QuiesceTableCounts(countMap)

	for i, vol := range volDescs {
		CreateResource(t, resources.PVs, vol.name,
			func() (string, error) {
				return kubectl.CreatePV(vol.name, vol.size, vol.accessModes)
			},
			func(r resourceAttrs) string {
				return fmt.Sprintf("SELECT storage, access_modes FROM pv "+
					"WHERE uid LIKE '%s'", r.uid)
			},
			[]interface{}{kubectl.GetStorageValue(vol.size),
				mysql.GetAccessModeString(vol.accessModes)},
			[]reflect.Type{tu.Int64Type, tu.StringType},
			countMap[dbmanager.PV]+1+i,
			fmt.Sprintf("PV for access mode %s",
				mysql.GetAccessModeString(vol.accessModes)),
		)
		pvcAttrs[i] = CreateResource(t, resources.PVCs, vol.name,
			func() (string, error) {
				return kubectl.CreatePVC(vol.name, watcherNS, vol.size,
					vol.accessModes)
			},
			func(r resourceAttrs) string {
				return fmt.Sprintf("SELECT storage, access_modes FROM pvc "+
					"WHERE uid LIKE '%s'", r.uid)
			},
			[]interface{}{kubectl.GetStorageValue(vol.size),
				mysql.GetAccessModeString(vol.accessModes)},
			[]reflect.Type{tu.Int64Type, tu.StringType},
			countMap[dbmanager.PVC]+1+i,
			fmt.Sprintf("PV for access mode %s",
				mysql.GetAccessModeString(vol.accessModes)),
		)
	}

	podAttrs := CreateResource(t, resources.Pods, "rw-mount-pod",
		func() (string, error) {
			return kubectl.CreatePod("rw-mount-pod", watcherNS, containerDescs)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT name FROM pod WHERE uid LIKE '%s'",
				r.uid)
		},
		[]interface{}{"rw-mount-pod"},
		[]reflect.Type{tu.StringType},
		countMap[dbmanager.Pod]+1, "ReadWrite mount test pod",
	)
	for i, pvc := range pvcAttrs {
		correct := tu.ValidateResult(t,
			fmt.Sprintf("SELECT container_name, pvc_name, read_only FROM "+
				"pod_mount WHERE pod_uid LIKE '%s' AND pvc_uid LIKE '%s'",
				podAttrs.uid, pvc.uid),
			[]interface{}{containerDescs[i].Name, volDescs[i].name,
				containerDescs[i].PVCMounts[0].ReadOnly},
			[]reflect.Type{tu.StringType, tu.StringType, tu.BoolType},
		)
		if !correct {
			t.Error("Failed to validate PVC ", i)
		}
	}
}

func TestLatePVC(t *testing.T) {
	var pvAttrs, pvcAttrs, podAttrs resourceAttrs
	var (
		pvName        = "nfs"
		pvcName       = "nfs"
		podName       = "vol-test"
		readOnlyMount = false

		size = 1

		podContainers = kubectl.GetContainerDescs(
			[][]resources.VolumeMount{
				[]resources.VolumeMount{
					resources.VolumeMount{
						Name: pvcName, ReadOnly: readOnlyMount,
					},
				},
			},
		)
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	countMap := GetStartingCounts()
	watcher.Watch(resources.PVs, false)
	watcher.Watch(resources.PVCs, false)
	watcher.Watch(resources.Pods, false)
	QuiesceTableCounts(countMap)

	pvAttrs = CreateResource(t, resources.PVs, pvName,
		func() (string, error) {
			return kubectl.CreatePV(pvName, size, defaultPVAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT p.storage, p.access_modes "+
				"FROM pv p WHERE p.uid LIKE '%s'", r.uid)
		},
		[]interface{}{kubectl.GetStorageValue(size),
			mysql.GetAccessModeString(defaultPVAccessModes),
		},
		[]reflect.Type{tu.Int64Type, tu.StringType},
		countMap[dbmanager.PV]+1, "PV")

	podAttrs = CreateResource(t, resources.Pods, podName,
		func() (string, error) {
			return kubectl.CreatePod(podName, watcherNS, podContainers)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT name, delete_time, "+
				"namespace FROM pod WHERE uid LIKE '%s'", r.uid)
		},
		[]interface{}{podName, nil, watcherNS},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.StringType},
		countMap[dbmanager.Pod]+1, "NFS pod")

	pvcAttrs = CreateResource(t, resources.PVCs, pvcName,
		func() (string, error) {
			return kubectl.CreatePVC(pvcName, watcherNS, size,
				defaultPVCAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT name, delete_time, "+
				"namespace, storage, access_modes, pv_uid FROM pvc WHERE uid "+
				"LIKE '%s'", r.uid)
		},
		[]interface{}{pvcName, nil, watcherNS, size * 1024 * 1024,
			mysql.GetAccessModeString(defaultPVCAccessModes),
			string(pvAttrs.uid)},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.StringType, tu.IntType,
			tu.StringType, tu.StringType},
		countMap[dbmanager.PVC]+1, "PVC")
	ValidateOperationTime(t, dbmanager.PVC, "bind", "bind_time",
		pvcAttrs.uid, pvcAttrs.createTime)

	CheckCount(t, dbmanager.PodMount, "pod_mount",
		countMap[dbmanager.PodMount]+1)
	correct := tu.ValidateResult(t,
		"SELECT pod_uid, pvc_uid, container_name, pvc_name, read_only "+
			"FROM pod_mount WHERE pod_uid LIKE '"+podAttrs.uid+"'",
		[]interface{}{podAttrs.uid, pvcAttrs.uid, podContainers[0].Name,
			pvcName, readOnlyMount},
		[]reflect.Type{tu.StringType, tu.StringType, tu.StringType,
			tu.StringType, tu.BoolType},
	)
	if !correct {
		t.Error("Incorrect entry found in pod_mount table")
	}
}

// Tests that we're storing PV(C) capacity properly.
func TestLargePV(t *testing.T) {
	var (
		largePVSize  = 8 * 1024
		largePVCSize = 7 * 1024
		pvName       = "large-nfs"
		pvcName      = "large-nfs"
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	countMap := GetStartingCounts()
	watcher.Watch(resources.PVs, false)
	watcher.Watch(resources.PVCs, false)
	QuiesceTableCounts(countMap)

	CreateResource(t, resources.PVs, pvName,
		func() (string, error) {
			return kubectl.CreatePV(pvName, largePVSize, defaultPVAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT storage FROM pv WHERE uid LIKE '%s'",
				r.uid)
		},
		[]interface{}{int64(largePVSize) * 1024 * 1024},
		[]reflect.Type{tu.Int64Type},
		countMap[dbmanager.PV]+1, "large PV")

	CreateResource(t, resources.PVCs, pvcName,
		func() (string, error) {
			return kubectl.CreatePVC(pvcName, watcherNS, largePVCSize,
				defaultPVCAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT storage FROM pvc WHERE uid LIKE '%s'",
				r.uid)
		},
		[]interface{}{int64(largePVCSize) * 1024 * 1024},
		[]reflect.Type{tu.Int64Type},
		countMap[dbmanager.PVC]+1, "large PVC")
}

func TestGetRV(t *testing.T) {
	var dbRV, watcherRV, ns string
	tu.InitDB(testDB)
	defer tu.DestroyDB()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	for _, resource := range []resources.ResourceType{resources.Pods,
		resources.PVs, resources.PVCs} {

		if resource == resources.PVs {
			ns = resources.PVNamespace
		} else {
			ns = watcherNS
		}
		dbRV = GetRV(resource, ns)
		watcherRV = watcher.getRV(resource, false)
		if dbRV != watcherRV {
			t.Errorf("DB RV for %s differs from watcher RV; expected %s, got "+
				"%s\n", resource, dbRV, watcherRV)
		}
	}
	for _, resource := range []resources.ResourceType{resources.Pods,
		resources.PVs, resources.PVCs} {

		watcherRV = watcher.getRV(resource, true)
		if watcherRV != "" {
			t.Errorf("Expected empty RV when specifying initialize with %s;"+
				" got %s\n", resource, watcherRV)
		}
	}

	altWatcher := GetMySQLWatcherForNamespace("unused-namespace")
	defer altWatcher.Destroy()
	for _, resource := range []resources.ResourceType{resources.Pods,
		resources.PVCs} {

		watcherRV = altWatcher.getRV(resource, false)
		if watcherRV != "" {
			t.Errorf("Expected empty RV for unused namespace and resource %s; "+
				"got %s\n", resource, watcherRV)
		}
	}
	dbRV = GetRV(resources.PVs, resources.PVNamespace)
	watcherRV = altWatcher.getRV(resources.PVs, false)
	if watcherRV != dbRV {
		t.Errorf("Expected %s for PVs from unused namespace watcher; got %s\n",
			dbRV, watcherRV)
	}
}

func TestResumption(t *testing.T) {
	var podUID string
	var podCreate time.Time

	var (
		podName       = "busybox"
		podContainers = kubectl.GetContainerDescs(
			[][]resources.VolumeMount{[]resources.VolumeMount{}})
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	countMap := GetStartingCounts()
	watcher.Watch(resources.Pods, false)
	QuiesceTableCounts(countMap)

	err := watcher.Stop(resources.Pods)
	if err != nil {
		log.Fatal("Error stopping watcher; something's wrong: ", err)
	}
	startTime := time.Now().Truncate(time.Second)
	if fileName, err := kubectl.CreatePod(podName, watcherNS,
		podContainers); err != nil {
		t.Fatalf("Unable to create %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	watcher.Watch(resources.Pods, false)
	//time.Sleep(time.Second)
	podUID, podCreate = GetMostRecentUIDTime(t, podName, resources.Pods)
	CheckCount(t, dbmanager.Pod, "pod created offline",
		countMap[dbmanager.Pod]+1)
	correct := tu.ValidateResult(t,
		"SELECT name, delete_time, namespace"+
			" FROM pod WHERE uid LIKE '"+podUID+"'",
		[]interface{}{podName, nil, watcherNS},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.StringType},
	)
	if !correct {
		t.Error("Got incorrect value(s) for pod created offline")
	}
	if podCreate.Before(startTime) {
		t.Errorf("Offline pod time error:\n\tDB create time: %s\n\t"+
			"Local time:  %s", podCreate, startTime)
	}
}

func TestLatePVCResumption(t *testing.T) {
	var (
		pvName        = "nfs"
		pvcName       = "nfs"
		podName       = "late-pvc-resumption-test"
		podName2      = "late-pvc-resumption-test-2"
		readOnlyMount = false

		size = 1

		podContainers = kubectl.GetContainerDescs(
			[][]resources.VolumeMount{
				[]resources.VolumeMount{
					resources.VolumeMount{
						Name: pvcName, ReadOnly: readOnlyMount,
					},
				},
			},
		)
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	countMap := GetStartingCounts()
	watcher.Watch(resources.PVs, false)
	watcher.Watch(resources.PVCs, false)
	watcher.Watch(resources.Pods, false)
	QuiesceTableCounts(countMap)

	err := watcher.Stop(resources.Pods)
	if err != nil {
		log.Fatal("Error stopping pod watcher; something's wrong: ", err)
	}
	err = watcher.Stop(resources.PVCs)
	if err != nil {
		log.Fatal("Error stopping pvc watcher; something's wrong: ", err)
	}
	err = watcher.Stop(resources.PVs)
	if err != nil {
		log.Fatal("Error stopping pv watcher; something's wrong: ", err)
	}

	if fileName, err := kubectl.CreatePod(podName, watcherNS,
		podContainers); err != nil {
		t.Fatalf("Unable to create %s:  %s", fileName, err)
	}
	if fileName, err := kubectl.CreatePod(podName2, watcherNS,
		podContainers); err != nil {
		t.Fatalf("Unable to create %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)

	if fileName, err := kubectl.CreatePV(pvName, size, defaultPVAccessModes); err != nil {
		t.Fatalf("Unable to create pv %s:  %s", fileName, err)
	}
	if fileName, err := kubectl.CreatePVC(
		pvcName, watcherNS, size, defaultPVCAccessModes,
	); err != nil {
		t.Fatalf("Unable to create pvc %s:  %s", fileName, err)
	}

	time.Sleep(time.Second)
	watcher.Watch(resources.PVs, false)
	watcher.Watch(resources.PVCs, false)
	watcher.Watch(resources.Pods, false)

	podUID1, _ := GetMostRecentUIDTime(t, podName, resources.Pods)
	podUID2, _ := GetMostRecentUIDTime(t, podName2, resources.Pods)
	pvcUID, _ := GetMostRecentUIDTime(t, pvcName, resources.PVCs)

	if correct := tu.ValidateResult(t,
		"SELECT pvc_name FROM pod_mount WHERE pod_uid LIKE '"+podUID1+"' AND "+
			"pvc_uid LIKE '"+pvcUID+"'",
		[]interface{}{pvcName},
		[]reflect.Type{tu.StringType},
	); !correct {
		t.Error("Failed to record pod mount to PVC when resuming.")
	}

	if correct := tu.ValidateResult(t,
		"SELECT pvc_name FROM pod_mount WHERE pod_uid LIKE '"+podUID2+"' AND "+
			"pvc_uid LIKE '"+pvcUID+"'",
		[]interface{}{pvcName},
		[]reflect.Type{tu.StringType},
	); !correct {
		t.Error("Failed to record second pod mount to PVC when resuming.")
	}
}

func TestPreexistingBind(t *testing.T) {
	var pvcUID, pvUID string
	var (
		pvName  = "preexisting-nfs"
		pvcName = "preexisting-nfs"
		size    = 1
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	watcher := GetMySQLWatcher()
	defer watcher.Destroy()

	if fileName, err := kubectl.CreatePV(pvName, size, defaultPVAccessModes); err != nil {
		t.Fatalf("Unable to create pv %s:  %s", fileName, err)
	}
	if fileName, err := kubectl.CreatePVC(
		pvcName, watcherNS, size, defaultPVCAccessModes,
	); err != nil {
		t.Fatalf("Unable to create pvc %s:  %s", fileName, err)
	}

	time.Sleep(time.Second)

	// We want to initialize here, since we're caring about events that happened
	// before Maestro was first brought up.
	watcher.Watch(resources.PVs, true)
	watcher.Watch(resources.PVCs, true)

	//time.Sleep(time.Second)

	pvUID, _ = GetMostRecentUIDTime(t, pvName, resources.PVs)
	pvcUID, _ = GetMostRecentUIDTime(t, pvcName, resources.PVCs)
	correct := tu.ValidateResult(t,
		"SELECT pv_uid FROM pvc WHERE uid LIKE '"+string(pvcUID)+"'",
		[]interface{}{string(pvUID)},
		[]reflect.Type{tu.StringType},
	)
	if !correct {
		t.Error("Got incorrect PVC UID for offline binding.")
	}
}

func TestMySQLPVUpdate(t *testing.T) {
	var (
		pvName      = "nfs-update"
		initialSize = 1
		newSize     = 5
		newExport   = "export-subdir"

		newPVAccessModes = []api.PersistentVolumeAccessMode{api.ReadWriteMany,
			api.ReadOnlyMany}
	)
	var nfsID, newNFSID int

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	w := GetMySQLWatcher()
	defer w.Destroy()

	countMap := GetStartingCounts()
	w.Watch(resources.PVs, false)
	QuiesceTableCounts(countMap)

	pvAttrs := CreateResource(t, resources.PVs, pvName,
		func() (string, error) {
			return kubectl.CreatePV(pvName, initialSize, defaultPVAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf(
				"SELECT name, storage, access_modes FROM pv WHERE uid "+
					"LIKE '%s'", r.uid,
			)
		},
		[]interface{}{pvName, kubectl.GetStorageValue(initialSize),
			mysql.GetAccessModeString(defaultPVAccessModes)},
		[]reflect.Type{tu.StringType, tu.Int64Type, tu.StringType},
		countMap[dbmanager.PV]+1, "PV for update",
	)
	err := tu.DB.QueryRow(
		fmt.Sprintf("SELECT nfs_id FROM pv WHERE uid LIKE '%s'", pvAttrs.uid),
	).Scan(&nfsID)
	if err != nil {
		t.Fatal("Unable to get NFS ID for newly created PV")
	}

	if fileName, err := kubectl.UpdatePV(pvName, newSize, newPVAccessModes); err != nil {
		t.Fatalf("Unable to update %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	if correct := tu.ValidateResult(t,
		fmt.Sprintf("SELECT name, nfs_id, storage, access_modes FROM pv WHERE "+
			"uid LIKE '%s'", pvAttrs.uid),
		[]interface{}{pvName, nfsID, kubectl.GetStorageValue(newSize),
			mysql.GetAccessModeString(newPVAccessModes)},
		[]reflect.Type{tu.StringType, tu.IntType, tu.Int64Type, tu.StringType},
	); !correct {
		t.Errorf("PV update failed.")
	}

	if fileName, err := kubectl.UpdatePVAtExport(
		pvName, newExport, newSize, newPVAccessModes,
	); err != nil {
		t.Fatalf("Unable to update %s with new backend:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	if correct := tu.ValidateResult(t,
		fmt.Sprintf("SELECT name, storage, access_modes FROM pv WHERE uid "+
			"LIKE '%s'", pvAttrs.uid),
		[]interface{}{pvName, kubectl.GetStorageValue(newSize),
			mysql.GetAccessModeString(newPVAccessModes)},
		[]reflect.Type{tu.StringType, tu.Int64Type, tu.StringType},
	); !correct {
		t.Errorf("PV update for backend touched unexpected fields.")
	}
	err = tu.DB.QueryRow(
		fmt.Sprintf("SELECT nfs_id FROM pv WHERE uid LIKE '%s'", pvAttrs.uid),
	).Scan(&newNFSID)
	if err != nil {
		t.Fatal("Unable to get NFS ID for newly created PV")
	}
	if newNFSID == nfsID {
		t.Error("Expected new NFS ID after update; got ", nfsID)
	}
	if correct := tu.ValidateResult(t,
		fmt.Sprintf("SELECT path FROM nfs WHERE id LIKE %d", newNFSID),
		[]interface{}{kubectl.GetExportPath(newExport)},
		[]reflect.Type{tu.StringType},
	); !correct {
		t.Error("Updated PV points to wrong backend.")
	}
}

func TestMySQLPVCUpdate(t *testing.T) {
	var (
		pvcName           = "nfs-update"
		initialSize       = 1
		newSize           = 5
		newPVCAccessModes = []api.PersistentVolumeAccessMode{api.ReadOnlyMany}
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	w := GetMySQLWatcher()
	defer w.Destroy()

	countMap := GetStartingCounts()
	w.Watch(resources.PVCs, false)
	QuiesceTableCounts(countMap)

	pvcAttrs := CreateResource(t, resources.PVCs, pvcName,
		func() (string, error) {
			return kubectl.CreatePVC(pvcName, watcherNS, initialSize,
				defaultPVCAccessModes)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf(
				"SELECT name, storage, access_modes FROM pvc WHERE uid LIKE "+
					"'%s'", r.uid,
			)
		},
		[]interface{}{pvcName, kubectl.GetStorageValue(initialSize),
			mysql.GetAccessModeString(defaultPVCAccessModes)},
		[]reflect.Type{tu.StringType, tu.Int64Type, tu.StringType},
		countMap[dbmanager.PVC]+1, "PVC for update",
	)

	if fileName, err := kubectl.UpdatePVC(
		pvcName, watcherNS, newSize, newPVCAccessModes,
	); err != nil {
		t.Fatalf("Unable to update %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	if correct := tu.ValidateResult(t,
		fmt.Sprintf("SELECT name, storage, access_modes FROM pvc WHERE uid "+
			"LIKE '%s'", pvcAttrs.uid),
		[]interface{}{pvcName, kubectl.GetStorageValue(newSize),
			mysql.GetAccessModeString(newPVCAccessModes)},
		[]reflect.Type{tu.StringType, tu.Int64Type, tu.StringType},
	); !correct {
		t.Errorf("PVC update failed.")
	}
}

func TestISCSIPV(t *testing.T) {
	const (
		pvName = "iscsi-pv"
		size1  = 1
		size2  = 2
		size3  = 3
		lun1   = 0
		lun2   = 1
	)
	var (
		iscsiID    int
		newISCSIID int
	)

	tu.InitDB(testDB)
	defer tu.DestroyDB()
	kubectl.DeleteTestResources()
	w := GetMySQLWatcher()
	defer w.Destroy()

	countMap := GetStartingCounts()
	w.Watch(resources.PVs, false)
	QuiesceTableCounts(countMap)

	pvAttrs := CreateResource(t, resources.PVs, pvName,
		func() (string, error) {
			return kubectl.CreateISCSIPV(pvName, size1, lun1)
		},
		func(r resourceAttrs) string {
			return fmt.Sprintf("SELECT p.name, p.delete_time, "+
				"p.storage, p.access_modes, p.nfs_id, i.target_portal, "+
				"i.iqn, i.lun, i.fs_type FROM pv p, iscsi i WHERE "+
				"p.iscsi_id = i.id and p.uid LIKE '%s'", r.uid)
		},
		[]interface{}{pvName, nil, kubectl.GetStorageValue(size1),
			mysql.GetAccessModeString(kubectl.GetISCSIAccessModes()), 0,
			kubectl.GetTargetPortal(), kubectl.GetIQN(), lun1,
			kubectl.GetFSType()},
		[]reflect.Type{tu.StringType, tu.TimeType, tu.Int64Type, tu.StringType,
			tu.IntType, tu.StringType, tu.StringType, tu.IntType,
			tu.StringType},
		countMap[dbmanager.PV]+1, "PV",
	)
	if err := tu.DB.QueryRow(
		fmt.Sprintf("SELECT iscsi_id FROM pv WHERE uid LIKE '%s'", pvAttrs.uid),
	).Scan(&iscsiID); err != nil {
		t.Fatal("Unable to retrieve iscsi ID for PV:  ", err)
	}

	// Update changing the backing volume (specifically, the lun number)
	if fileName, err := kubectl.UpdateISCSIPV(pvName, size2, lun2); err != nil {
		t.Fatalf("Unable to upate %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	if correct := tu.ValidateResult(t,
		fmt.Sprintf("SELECT p.storage, i.lun FROM pv p, iscsi i WHERE "+
			"p.iscsi_id = i.id AND p.uid LIKE '%s'", pvAttrs.uid),
		[]interface{}{kubectl.GetStorageValue(size2), lun2},
		[]reflect.Type{tu.Int64Type, tu.IntType},
	); !correct {
		t.Error("ISCSI PV updated incorrectly.")
	}
	if err := tu.DB.QueryRow(
		fmt.Sprintf("SELECT iscsi_id FROM pv WHERE uid LIKE '%s'", pvAttrs.uid),
	).Scan(&newISCSIID); err != nil {
		t.Fatal("Unable to retrieve iscsi ID for PV:  ", err)
	}
	if newISCSIID == iscsiID {
		t.Error("ISCSI ID not updated; got ", iscsiID)
	}

	// Update without changing the backing volume.
	if fileName, err := kubectl.UpdateISCSIPV(pvName, size2, lun2); err != nil {
		t.Fatalf("Unable to upate %s:  %s", fileName, err)
	}
	time.Sleep(time.Second)
	if correct := tu.ValidateResult(t,
		fmt.Sprintf("SELECT storage, iscsi_id FROM pv WHERE uid LIKE '%s'",
			pvAttrs.uid),
		[]interface{}{kubectl.GetStorageValue(size2), newISCSIID},
		[]reflect.Type{tu.Int64Type, tu.IntType},
	); !correct {
		t.Error("Update with same backend failed.")
	}
}
