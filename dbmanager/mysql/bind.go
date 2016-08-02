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

func (m *mySQLManager) initBindStatements() (err error) {

	m.bindStatements = make(map[dbmanager.Table]*sql.Stmt)

	m.bindStatements[dbmanager.PVC], err = m.db.Prepare(
		"INSERT INTO pvc (uid, pv_uid, bind_time) VALUES (?, ?, ?) " +
			"ON DUPLICATE KEY UPDATE pv_uid = ?, bind_time = ?",
	)
	if err != nil {
		log.Print("Unable to create PVC bind statement: ", err)
		delete(m.bindStatements, dbmanager.PVC)
		return // Currently superfluous, but this function may grow.
	}

	return
}

func (m *mySQLManager) destroyBindStatements() {
	if m.bindStatements == nil {
		return
	}
	for _, stmt := range m.bindStatements {
		stmt.Close()
	}
}

func (m *mySQLManager) BindPVC(pvUID types.UID, pvcUID types.UID,
	bindTime unversioned.Time, rv string) {

	var err error

	err = m.runTx(
		func(tx *sql.Tx) error {
			rows, err := m.doTxStatementCheckRows(tx, "bind", dbmanager.PVC,
				m.bindStatements, string(pvcUID), string(pvUID),
				bindTime.Time, string(pvUID), bindTime.Time)
			if err != nil {
				return err
			}
			if rows < 1 || rows > 2 {
				return fmt.Errorf("Bind for PVC uid %s, PV uid %s, at time %s"+
					" affected unexpected number of rows:  %d\n", pvcUID,
					pvUID, bindTime.Time, rows)
			}
			// This looks odd, but the resource version will correspond to the
			// RV of the PV, not the PVC.
			return m.updateRV(tx, resources.PVs, resources.PVNamespace, rv)
		},
	)
	if err != nil {
		log.Fatal("Unable to update PVCs:\n\t", err)
	}
}
