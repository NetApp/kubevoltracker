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
	"testing"
)

func TestExistence(t *testing.T) {
	manager.clearTestTables()

	nfs_id := manager.InsertNFS("127.0.0.1", "/path")

	tx, err := manager.db.Begin()
	if err != nil {
		log.Fatal("Unable to start transaction: ", err)
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()
	new_nfs_id := manager.checkNFSExists(tx, "127.0.0.1", "/path")
	if nfs_id != new_nfs_id {
		t.Errorf("Retrieved wrong ID for NFS entry 127.0.0.1, /path.\n\t"+
			"Got: %d; expected: %d\n", new_nfs_id, nfs_id)
	}
	wrong_path_id := manager.checkNFSExists(tx, "127.0.0.1", "/nope")
	if wrong_path_id != -1 {
		t.Errorf("Got nonnegative ID for nonexistent path 127.0.0.1/nope: %d\n",
			wrong_path_id)
	}
	wrong_ip_id := manager.checkNFSExists(tx, "127.0.0.2", "/path")
	if wrong_ip_id != -1 {
		t.Errorf("Got nonnegative ID for nonexistent path 127.0.0.2/path: %d\n",
			wrong_ip_id)
	}
	wrong_both_id := manager.checkNFSExists(tx, "127.0.0.2", "/nope")
	if wrong_both_id != -1 {
		t.Errorf("Got nonnegative ID for nonexistent path 127.0.0.2/nope: %d\n",
			wrong_ip_id)
	}
	tx.Commit()
	tx = nil

	duplicate_nfs_id := manager.InsertNFS("127.0.0.1", "/path")
	if duplicate_nfs_id != nfs_id {
		t.Errorf("Inserted duplicate NFS record with ID %d; expected "+
			"redundancy with ID %d\n", duplicate_nfs_id, nfs_id)
	}
}
