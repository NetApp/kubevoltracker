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
	"database/sql"
	"fmt"
	"log"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/types"

	"github.com/netapp/kubevoltracker/dbmanager"
	"github.com/netapp/kubevoltracker/resources"
)

// Used to initialize mySQLManager's insert statements.  m.db MUST be
// initialized.
func (m *mySQLManager) initInsertStatements() (err error) {
	var insertStmt *sql.Stmt

	m.insertStatements = make(map[dbmanager.Table]*sql.Stmt)

	insertStmt, err = m.db.Prepare("INSERT INTO pod (uid, name, create_time, " +
		"namespace, json) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Print("Unable to create pod insert statement: ", err)
		return
	}
	m.insertStatements[dbmanager.Pod] = insertStmt

	// The second clause is necessary because this row may already have been
	// created thanks to a PV entering the bound state before the creation event
	// for this PVC is processed (hurray for asynchronicity!)
	insertStmt, err = m.db.Prepare("INSERT INTO pvc (uid, name, create_time, " +
		"namespace, storage, access_modes, json) VALUES (?, ?, ?, ?, ?, ?, ?)" +
		" ON DUPLICATE KEY UPDATE name=?, create_time = ?, namespace = ?, " +
		"storage = ?, access_modes = ?, json = ?")
	if err != nil {
		log.Print("Unable to create PVC insert statement: ", err)
		return err
	}
	m.insertStatements[dbmanager.PVC] = insertStmt

	insertStmt, err = m.db.Prepare("INSERT INTO pv (uid, name, create_time, " +
		"storage, access_modes, json, nfs_id, iscsi_id) VALUES (?, ?, ?, ?, " +
		"?, ?, ?, ?)")
	if err != nil {
		log.Print("Unable to create PV insert statement: ", err)
		return err
	}
	m.insertStatements[dbmanager.PV] = insertStmt

	insertStmt, err = m.db.Prepare("INSERT INTO nfs (ip_addr, path) " +
		" VALUES (inet_aton(?), ?)")
	if err != nil {
		log.Print("Unable to create NFS insert statement: ", err)
		return err
	}
	m.insertStatements[dbmanager.NFS] = insertStmt

	// Note that this doesn't use inet_aton for target_portal since
	// target_portal may contain a port number.
	insertStmt, err = m.db.Prepare("INSERT INTO iscsi(target_portal, iqn, " +
		"lun, fs_type) VALUES(?, ?, ?, ?)")
	if err != nil {
		log.Print("Unable to create ISCSI insert statement: ", err)
		return err
	}
	m.insertStatements[dbmanager.ISCSI] = insertStmt

	insertStmt, err = m.db.Prepare(
		"INSERT INTO pod_mount (pod_uid, pvc_uid, container_name, pvc_name, " +
			"read_only) SELECT ?, uid, ?, ?, ? " +
			"FROM pvc WHERE create_time = (SELECT MAX(create_time) FROM pvc " +
			"WHERE create_time <= ? and " +
			"(delete_time IS NULL OR delete_time >= ?)" +
			" and name like ?) and name like ?;")
	if err != nil {
		log.Print("Unable to create PodMounts insert statement: ", err)
		return err
	}
	m.insertStatements[dbmanager.PodMount] = insertStmt

	insertStmt, err = m.db.Prepare(
		"INSERT INTO pod_mount (pod_uid, pvc_uid, container_name, pvc_name, " +
			"read_only) SELECT ?, uid, ?, ?, ? " +
			"FROM pvc WHERE create_time = (SELECT MIN(create_time) FROM pvc " +
			"WHERE create_time >= ? AND " +
			"(delete_time IS NULL OR delete_time > ?)" +
			" AND name LIKE ?) AND name LIKE ?;")
	if err != nil {
		log.Print("Unable to create PodMount insert statement for PVCs "+
			"created after the pod: ", err)
		return err
	}
	m.latePVCPodMount = insertStmt

	insertStmt, err = m.db.Prepare(
		"INSERT INTO pod_mount (pod_uid, container_name, pvc_name, read_only)" +
			" VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Print("Unable to create fallback PodMounts insert statement:  ",
			err)
		return err
	}
	m.fallbackPodMount = insertStmt

	// We need to update all entries corresponding to pods that haven't been
	// deleted, since they may match this PVC.  We're assuming that
	// users don't delete PVCs that are currently in use, so we won't see
	// a legit entry get clobbered.  We may, however, see erroneous entries
	// for a bit while things catch up.
	// NOTE THAT THIS BREAKS IF WE LOSE EVENTS.
	m.addPVCPodMount, err = m.db.Prepare("UPDATE pod_mount m, pod p SET " +
		"m.pvc_uid = ? WHERE m.pvc_name LIKE ? and m.pod_uid = p.uid AND " +
		"p.delete_time IS NULL")
	if err != nil {
		log.Print("Unable to create query to add a PVC UID to an existing",
			"pod_mount entry: ", err)
		m.addPVCPodMount = nil
		return err
	}

	insertStmt, err = m.db.Prepare(
		"INSERT INTO container (pod_uid, name, image, command) VALUES " +
			"(?, ?, ?, ?)",
	)
	if err != nil {
		log.Print("Unable to create container insert statement: ", err)
		return err
	}
	m.insertStatements[dbmanager.Container] = insertStmt

	return nil
}

func (m *mySQLManager) destroyInsertStatements() {
	if m.insertStatements == nil {
		return
	}
	for _, insertStmt := range m.insertStatements {
		insertStmt.Close()
	}
}

func (m *mySQLManager) insertPodMount(tx *sql.Tx, uid types.UID,
	containerName string, pvcMount resources.VolumeMount,
	createTime unversioned.Time) error {

	// The insert statement for PodMount takes as parameters the pod
	// uid, the pod's creation time, and the PVC's name and finds the
	// PVC created closest to the pod's creation time without exceeding
	// it.
	rows, err := m.doTxStatementCheckRows(tx, "insert",
		dbmanager.PodMount, m.insertStatements, string(uid), containerName,
		pvcMount.Name, pvcMount.ReadOnly, createTime.Time, createTime.Time,
		pvcMount.Name, pvcMount.Name)
	if err != nil {
		return err
	}
	if rows > 1 {
		err = fmt.Errorf("Pod mount insert updated %d rows; "+
			"this is not expected", rows)
		return err
	}
	if rows == 1 {
		// Everything worked as planned.
		return nil
	}

	// See if the PVC was created after the pod.
	result, err := tx.Stmt(m.latePVCPodMount).Exec(string(uid), containerName,
		pvcMount.Name, pvcMount.ReadOnly, createTime.Time, createTime.Time,
		pvcMount.Name, pvcMount.Name)
	if err != nil {
		return err
	}
	rows, err = result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("Unable to retrieve number of rows "+
			"printed:  %s\n", err)
		return err
	}
	if rows > 1 {
		err = fmt.Errorf("Pod mount insert updated %d rows; "+
			"this is not expected", rows)
		return err
	}
	if rows == 1 {
		return nil
	}

	// If we get to this point, we haven't seen the PVC yet.
	result, err = tx.Stmt(m.fallbackPodMount).Exec(string(uid),
		containerName, pvcMount.Name, pvcMount.ReadOnly)
	if err != nil {
		err = fmt.Errorf("Unable to insert partial record into "+
			"pod mount table for pod %s, pvc %s:  %v\n", uid,
			pvcMount.Name, err)
		return err
	}
	rows, err = result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("Unable to retrieve number of rows "+
			"printed:  %s\n", err)
		return err
	} else if rows != 1 {
		err = fmt.Errorf("Fallback pod mount affected unexpected "+
			"number of rows:  %d\n", rows)
		return err
	}
	return err
}

func (m *mySQLManager) InsertPod(uid types.UID, name string,
	createTime unversioned.Time, namespace string,
	containers []resources.ContainerDesc, json, watcherNS, rv string) {

	var err error

	log.Print("Adding pod with UID ", uid)

	err = m.runTx(func(tx *sql.Tx) error {
		if err = m.doTxStatement(tx, "insert", dbmanager.Pod,
			m.insertStatements, string(uid), name, createTime.Time, namespace,
			json); err != nil {
			return err
		}
		for _, container := range containers {
			if err = m.doTxStatement(tx, "insert", dbmanager.Container,
				m.insertStatements, string(uid), container.Name,
				container.Image, container.Command); err != nil {
				return err
			}
			for _, pvc := range container.PVCMounts {
				m.insertPodMount(tx, uid, container.Name, pvc, createTime)
			}
		}
		err = m.updateRV(tx, resources.Pods, watcherNS, rv)
		if err != nil {
			log.Fatalf("Unable to update pod resource version in namespace %s:"+
				" %s\n", watcherNS, err)
		}
		return err
	})
	if err != nil {
		log.Fatal("Unable to create pod:\n\t", err)
	}
}

func (m *mySQLManager) InsertPV(uid types.UID, name string,
	createTime unversioned.Time, backendID int, backendType dbmanager.Table,
	storage int64, accessModes []api.PersistentVolumeAccessMode, json,
	rv string) {

	var err error
	var (
		nfsID   = 0
		iscsiID = 0
	)

	err = m.runTx(func(tx *sql.Tx) error {
		switch {
		case backendType == dbmanager.NFS:
			nfsID = backendID
		case backendType == dbmanager.ISCSI:
			iscsiID = backendID
		default:
			log.Fatal("Unknown backend type:  ", backendType)
		}
		if err = m.doTxStatement(tx, "insert",
			dbmanager.PV, m.insertStatements,
			string(uid), name, createTime.Time, storage,
			GetAccessModeString(accessModes), json, nfsID, iscsiID,
		); err != nil {
			return err
		}
		err = m.updateRV(tx, resources.PVs, resources.PVNamespace, rv)
		return err
	})
	if err != nil {
		log.Fatal("Unable to create PV:\n\t", err)
	}
}

func (m *mySQLManager) InsertPVC(uid types.UID, name string,
	createTime unversioned.Time, namespace string, storage int64,
	accessModes []api.PersistentVolumeAccessMode,
	json, watcherNS, rv string) {

	var err error

	modeString := GetAccessModeString(accessModes)

	err = m.runTx(func(tx *sql.Tx) error {
		rows, err := m.doTxStatementCheckRows(tx, "insert", dbmanager.PVC,
			m.insertStatements, string(uid), name, createTime.Time, namespace,
			storage, modeString, json, name, createTime.Time, namespace,
			storage, modeString, json)
		if err != nil {
			return err
		}
		if rows < 1 || rows > 2 {
			err = fmt.Errorf("PVC insert with uid %s affected unexpected "+
				"number of rows: %d\n", uid, rows)
		}
		_, err = tx.Stmt(m.addPVCPodMount).Exec(string(uid), name)
		if err != nil {
			err = fmt.Errorf("Unable to update PVC mount table:  %s", err)
			return err
		}
		err = m.updateRV(tx, resources.PVCs, watcherNS, rv)
		return err
	})
	if err != nil {
		log.Fatal("Unable to insert PVC:\n\t", err)
	}
}

// Note that we can't use runTx in this function, since it has a different
// return signature.
func (m *mySQLManager) InsertNFS(ipAddr, path string) int {
	var nfsID int

	tx, err := m.db.Begin()
	if err != nil {
		log.Fatal("Unable to create transaction: ", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
			return
		}
		err = tx.Commit()
		if err != nil {
			log.Fatal("Unable to commit transaction: ", err)
		}
	}()
	nfsID = m.checkNFSExists(tx, ipAddr, path)
	if nfsID > 0 {
		return nfsID
	}
	if err := m.doTxStatement(tx, "insert", dbmanager.NFS,
		m.insertStatements, ipAddr, path); err != nil {
		return -1
	}
	row := tx.Stmt(m.lastIDQuery).QueryRow()
	err = row.Scan(&nfsID)
	if err != nil {
		err = fmt.Errorf("Unable to retrieve last created ID:  %s", err)
		return -1
	}
	return nfsID
}

func (m *mySQLManager) InsertISCSI(
	targetPortal, iqn string, lun int, fsType string,
) int {
	var err error
	var iscsiID int

	err = m.runTx(func(tx *sql.Tx) error {
		iscsiID = m.checkISCSIExists(tx, targetPortal, iqn, lun, fsType)
		if iscsiID > 0 {
			return nil
		}
		if err := m.doTxStatement(tx, "insert", dbmanager.ISCSI,
			m.insertStatements, targetPortal, iqn, lun, fsType); err != nil {
			return err
		}
		row := tx.Stmt(m.lastIDQuery).QueryRow()
		err = row.Scan(&iscsiID)
		if err != nil {
			err = fmt.Errorf("Unable to retrieve last created ID:  %s", err)
		}
		return err
	})
	if err != nil {
		log.Fatal("Unable to insert ISCSI record:\n\t", err)
	}
	return iscsiID
}
