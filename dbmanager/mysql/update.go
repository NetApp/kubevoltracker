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
	"log"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"

	"github.com/netapp/kubevoltracker/dbmanager"
	"github.com/netapp/kubevoltracker/resources"
)

func (m *mySQLManager) initUpdateStatements() error {
	var err error

	m.updateStatements = make(map[dbmanager.Table]*sql.Stmt)
	m.updateStatements[dbmanager.PV], err = m.db.Prepare(
		"UPDATE pv SET nfs_id = ?, iscsi_id = ?, storage = ?, " +
			"access_modes = ?, json = ? WHERE uid = ?")
	if err != nil {
		log.Print("Unable to create PV update statement: ", err)
		delete(m.updateStatements, dbmanager.PV)
		return err
	}
	m.updateStatements[dbmanager.PVC], err = m.db.Prepare(
		"UPDATE pvc SET storage = ?, access_modes = ?, json = ? WHERE uid = ?")
	if err != nil {
		log.Print("Unable to create PVC update statement: ", err)
		delete(m.updateStatements, dbmanager.PVC)
		return err
	}
	return nil
}

func (m *mySQLManager) destroyUpdateStatements() {
	if m.updateStatements == nil {
		return
	}
	for _, updateStmt := range m.updateStatements {
		updateStmt.Close()
	}
}

func (m *mySQLManager) UpdatePV(
	uid types.UID, backendID int, backendType dbmanager.Table, storage int64,
	accessModes []api.PersistentVolumeAccessMode, json, rv string,
) {
	var err error
	var (
		nfsID   = 0
		iscsiID = 0
	)

	switch {
	case backendType == dbmanager.NFS:
		nfsID = backendID
	case backendType == dbmanager.ISCSI:
		iscsiID = backendID
	default:
		log.Fatal("Unrecognized backend type for update:  ", backendType)
	}
	err = m.runTx(
		func(tx *sql.Tx) error {
			if err = m.doTxStatement(tx, "update", dbmanager.PV,
				m.updateStatements, nfsID, iscsiID, storage,
				GetAccessModeString(accessModes), json,
				string(uid),
			); err != nil {
				return err
			}
			err = m.updateRV(tx, resources.PVs, resources.PVNamespace, rv)
			return err
		},
	)
	if err != nil {
		log.Fatal("Unable to update PV:\n\t", err)
	}
}

func (m *mySQLManager) UpdatePVC(uid types.UID, storage int64,
	accessModes []api.PersistentVolumeAccessMode,
	json, watcherNS, rv string) {

	var err error
	err = m.runTx(
		func(tx *sql.Tx) error {
			if err = m.doTxStatement(tx, "update", dbmanager.PVC,
				m.updateStatements, storage, GetAccessModeString(accessModes),
				json, string(uid),
			); err != nil {
				return err
			}
			err = m.updateRV(tx, resources.PVCs, watcherNS, rv)
			return err
		},
	)
	if err != nil {
		log.Fatal("Unable to update PVC:\n\t", err)
	}
}
