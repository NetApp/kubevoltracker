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

package testutils

import (
	"log"
	"os"
	"reflect"
	"testing"
	"time"
)

// Test of the test
func TestValidate(t *testing.T) {
	test_time := time.Now()
	t.Logf("Inserting time %v\n", test_time)
	result, err := DB.Exec("INSERT INTO pv (uid, name, create_time, storage,"+
		"json) VALUES (?, ?, ?, 1024, '{}')", "test-validate-uid",
		"test-validate", test_time)
	if err != nil {
		log.Fatal("SQL failed to execute: ", err)
	}
	if rows, err := result.RowsAffected(); rows != 1 || err != nil {
		log.Fatalf("Affected unexpected number of rows %d or err:  %s\n",
			rows, err)
	}
	correct := ValidateResult(t, "SELECT uid, name, create_time, delete_time "+
		"FROM pv WHERE uid = 'test-validate-uid'",
		[]interface{}{"test-validate-uid", "test-validate", test_time, nil},
		[]reflect.Type{StringType, StringType, TimeType, TimeType})
	if !correct {
		t.Errorf("Failed to evaluate test results successfully.\n")
	}
	correct = ValidateResult(t, "SELECT uid, name, create_time, delete_time "+
		"FROM pv WHERE uid = 'test-validate-uid'",
		[]interface{}{"test-validate-uid", "incorrect", test_time, test_time},
		[]reflect.Type{StringType, StringType, TimeType, TimeType})
	if correct {
		t.Errorf("Failed to detect incorrect entries.\n")
	}
	t.Log("Validate test succeeded")
	log.Print("Ran validation test")
}

func runTxn(t *testing.T, ch chan error, podUID, podName, txnName string,
	sleepFirst bool) {

	tx, err := DB.Begin()
	if err != nil {
		t.Logf("Unable to start transaction %s:  %v\n", txnName, err)
		ch <- err
		return
	}
	log.Printf("Started transaction %s\n", txnName)
	if sleepFirst {
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("Transaction %s inserting\n", txnName)
	_, err = tx.Exec(
		"INSERT INTO pod (uid, name) VALUES (?, ?)",
		podUID,
		podName,
	)
	log.Printf("Transaction %s inserted\n", txnName)
	if err != nil {
		tx.Rollback()
		t.Logf("Transaction %s unable to insert into pod:  %v\n", txnName, err)
		ch <- err
		return
	}
	time.Sleep(2 * time.Second)
	log.Printf("Transaction %s committing\n", txnName)
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		t.Logf("Transaction %s unable to commit:  %v\n", txnName, err)
		ch <- err
		return
	}
	log.Printf("Transaction %s committed\n", txnName)
	ch <- nil
	return
}

func TestMain(m *testing.M) {
	InitDB("kubevoltracker_test")
	defer DestroyDB()
	os.Exit(m.Run())
}
