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

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/types"

	"github.com/netapp/kubevoltracker/dbmanager"
	"github.com/netapp/kubevoltracker/resources"
)

// m.db MUST be initialized before this is called.
func (m *mySQLManager) initDeleteStatements() (err error) {
	var deleteStmt *sql.Stmt

	m.deleteStatements = make(map[dbmanager.Table]*sql.Stmt)

	deleteStmt, err = m.db.Prepare("UPDATE pod SET delete_time=? WHERE " +
		"uid=?")
	if err != nil {
		log.Print("Error creating pod delete statement:  ", err)
		return
	}
	m.deleteStatements[dbmanager.Pod] = deleteStmt

	deleteStmt, err = m.db.Prepare("UPDATE pvc SET delete_time=? WHERE " +
		"uid=?")
	if err != nil {
		log.Print("Error creating pvc delete statement:  ", err)
		return
	}
	m.deleteStatements[dbmanager.PVC] = deleteStmt

	deleteStmt, err = m.db.Prepare("UPDATE pv SET delete_time=? WHERE " +
		"uid=?")
	if err != nil {
		log.Print("Error creating pv delete statement:  ", err)
		return
	}
	m.deleteStatements[dbmanager.PV] = deleteStmt

	deleteStmt, err = m.db.Prepare(
		"DELETE FROM pod_mount WHERE pod_uid = ? and pvc_uid IN " +
			"(SELECT uid FROM  pvc WHERE pvc.create_time > ?)",
	)
	if err != nil {
		log.Print("Error creating statement to clear incorrect pod mounts: ",
			err)
		return
	}
	m.clearBadPodMount = deleteStmt
	return
}

func (m *mySQLManager) DeletePod(uid types.UID, deleteTime unversioned.Time,
	watcher_ns, rv string) {
	var err error

	err = m.runTx(
		func(tx *sql.Tx) error {
			if err = m.doTxStatement(tx, "delete", dbmanager.Pod,
				m.deleteStatements, deleteTime.Time, string(uid)); err != nil {
				err = fmt.Errorf("Basic delete failed:  %s\n", err)
				return err
			}
			result, err := tx.Stmt(m.clearBadPodMount).Exec(string(uid),
				deleteTime.Time)
			if err != nil {
				err = fmt.Errorf("Failed to clear bad pod mounts:  %s\n", err)
				return err
			}
			rows, err := result.RowsAffected()
			if err != nil {
				err = fmt.Errorf("Unable to retrieve number of rows "+
					"printed:  %s\n", err)
				return err
			}
			if rows > 0 {
				// If we've deleted at least one row, we need to delete all
				// rows for that UID, since the pod never succeeded in
				// initializing.  Don't bother using a prepared statement,
				// since this should be quite rare.
				_, err = tx.Exec("DELETE FROM pod_mount WHERE pod_uid = ?",
					string(uid))
				if err != nil {
					err = fmt.Errorf("Final clean of pod entries failed:  %s\n",
						err)
					return err
				}
			}
			return m.updateRV(tx, resources.Pods, watcher_ns, rv)
		},
	)
	if err != nil {
		log.Fatal("Unable to delete pod:\n\t", err)
	}
}

func (m *mySQLManager) DeletePV(uid types.UID, deleteTime unversioned.Time,
	rv string) {

	var err error

	err = m.runTx(
		func(tx *sql.Tx) error {
			if err = m.doTxStatement(tx, "delete", dbmanager.PV,
				m.deleteStatements, deleteTime.Time, string(uid)); err != nil {
				return err
			}
			return m.updateRV(tx, resources.PVs, resources.PVNamespace, rv)
		},
	)
	if err != nil {
		log.Fatal("Unable to delete PV:\n\t", err)
	}
}

func (m *mySQLManager) DeletePVC(uid types.UID, deleteTime unversioned.Time,
	watcher_ns, rv string) {
	var err error

	err = m.runTx(
		func(tx *sql.Tx) error {
			if err = m.doTxStatement(tx, "delete", dbmanager.PVC,
				m.deleteStatements, deleteTime.Time, string(uid)); err != nil {
				return err
			}
			return m.updateRV(tx, resources.PVCs, watcher_ns, rv)
		},
	)
	if err != nil {
		log.Fatal("Unable to delete PV:\n\t", err)
	}
}

func (m *mySQLManager) destroyDeleteStatements() {
	if m.deleteStatements == nil {
		return
	}
	for _, deleteStmt := range m.deleteStatements {
		deleteStmt.Close()
	}
}
