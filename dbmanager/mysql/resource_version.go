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

	"github.com/netapp/kubevoltracker/resources"
)

func (m *mySQLManager) initRVQueries() (err error) {
	m.updateRVQuery, err = m.db.Prepare("INSERT INTO resource_version " +
		"(resource, namespace, resource_version) VALUES (?, ?, ?) ON " +
		"DUPLICATE KEY UPDATE resource_version=?")
	if err != nil {
		log.Print("Unable to create update resource version query: ", err)
		m.updateRVQuery = nil
		return
	}

	m.getRVQuery, err = m.db.Prepare("SELECT resource_version FROM " +
		"resource_version WHERE resource LIKE ? AND namespace LIKE ?")
	if err != nil {
		log.Print("Unable to create resource version get query: ", err)
		m.getRVQuery = nil
		return
	}
	return
}

func (m *mySQLManager) destroyRVQueries() {
	if m.updateRVQuery != nil {
		m.updateRVQuery.Close()
	}
	if m.getRVQuery != nil {
		m.getRVQuery.Close()
	}
}

func (m *mySQLManager) updateRV(tx *sql.Tx, resource resources.ResourceType,
	namespace, rv string) error {

	result, err := tx.Stmt(m.updateRVQuery).Exec(string(resource), namespace,
		rv, rv)
	if err != nil {
		return fmt.Errorf(
			"Unable to update %s in namespace %s with rv %s: %s\n",
			resource, namespace, rv, err)
	}
	// TODO:  Consider no longer checking the number of rows affected.
	rows, err := result.RowsAffected()
	// It's possible for a pod resource version to be updated multiple times
	// if there's more than one watcher going.  This isn't a problem.
	if (rows < 1 && resource != resources.PVs) || rows > 2 {
		log.Printf("Resource version update for %s, namespace %s, rv %s "+
			"affected unexpected number of rows: %d\n", resource, namespace,
			rv, rows)
	}
	return err
}

func (m *mySQLManager) GetRV(resource resources.ResourceType,
	namespace string) string {

	var rv string
	err := m.getRVQuery.QueryRow(string(resource), namespace).Scan(&rv)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		log.Panicf("Unable to get resource version for %s in namespace %s:  %s",
			resource, namespace, err)
	}
	return rv
}
