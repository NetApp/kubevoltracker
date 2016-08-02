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

	"github.com/netapp/kubevoltracker/dbmanager"
)

func (m *mySQLManager) initExistenceQueries() (err error) {
	m.existenceQueries = make(map[dbmanager.Table]*sql.Stmt)

	m.existenceQueries[dbmanager.NFS], err = m.db.Prepare(
		"SELECT id FROM nfs WHERE ip_addr = inet_aton(?) and path like ?",
	)
	if err != nil {
		log.Print("Unable to initialize NFS existence query: ", err)
		delete(m.existenceQueries, dbmanager.NFS)
		return
	}
	m.existenceQueries[dbmanager.ISCSI], err = m.db.Prepare(
		"SELECT id FROM iscsi WHERE target_portal LIKE ? AND iqn LIKE ? " +
			"AND lun = ? AND fs_type LIKE ?",
	)
	if err != nil {
		log.Print("Unable to initialize ISCSI existence query: ", err)
		delete(m.existenceQueries, dbmanager.ISCSI)
		return
	}
	return
}

func (m *mySQLManager) destroyExistenceQueries() {
	if m.existenceQueries == nil {
		return
	}
	for _, query := range m.existenceQueries {
		query.Close()
	}
}

func (m *mySQLManager) checkNFSExists(tx *sql.Tx, ipaddr string,
	path string) int {

	var nfs_id int
	err := tx.Stmt(m.existenceQueries[dbmanager.NFS]).QueryRow(ipaddr,
		path).Scan(&nfs_id)
	switch {
	case err == sql.ErrNoRows:
		return -1
	case err != nil:
		log.Fatal("Unable to query NFS table: ", err)
	default:
		return nfs_id
	}
	return 0
}

func (m *mySQLManager) checkISCSIExists(tx *sql.Tx, targetPortal, iqn string,
	lun int, fsType string) int {

	var iscsi_id int
	err := tx.Stmt(m.existenceQueries[dbmanager.ISCSI]).QueryRow(targetPortal,
		iqn, lun, fsType).Scan(&iscsi_id)
	switch {
	case err == sql.ErrNoRows:
		return -1
	case err != nil:
		log.Fatal("Unable to query ISCSI table: ", err)
	default:
		return iscsi_id
	}
	return 0
}
