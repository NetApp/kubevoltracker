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
	"testing"

	"github.com/netapp/kubevoltracker/resources"
)

func TestResourceVersion(t *testing.T) {
	manager.clearTestTables()

	var err error

	testRV1 := "1456"
	testRV2 := "1457"
	err = manager.runTx(
		func(tx *sql.Tx) error {
			err = manager.updateRV(tx, resources.Pods, watcher_ns_alt, testRV1)
			return err
		},
	)
	if err != nil {
		t.Fatal("Unable to update RV: ", err)
	}
	rv := manager.GetRV(resources.Pods, watcher_ns_alt)
	if rv != testRV1 {
		t.Errorf("Retrieved incorrect RV; expected %s, got %s", testRV1, rv)
	}
	// Check that update of existing key works.
	err = manager.runTx(
		func(tx *sql.Tx) error {
			err = manager.updateRV(tx, resources.Pods, watcher_ns_alt, testRV2)
			return err
		},
	)
	rv = manager.GetRV(resources.Pods, watcher_ns_alt)
	if rv != testRV2 {
		t.Errorf("Retrieved incorrect RV; expected %s, got %s", testRV2, rv)
	}
}
