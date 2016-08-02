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

// Package mysql provides a dbmanager implementation that uses a MySQL database
// for persistence.  See kubevoltracker/dbmanager for method documentation.
package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/netapp/kubevoltracker/dbmanager"
)

const (
	SQLTimeFormat = "2006-01-02 15:04:05"
	// Source for deadlockErrNo:
	// https://dev.mysql.com/doc/refman/5.6/en/error-messages-server.html
	deadlockErrNo = 1213
)

const maxTries = 5

const ErrDuplicateKey = 1062

type mySQLManager struct {
	db *sql.DB

	insertStatements map[dbmanager.Table]*sql.Stmt
	deleteStatements map[dbmanager.Table]*sql.Stmt
	bindStatements   map[dbmanager.Table]*sql.Stmt
	updateStatements map[dbmanager.Table]*sql.Stmt

	existenceQueries map[dbmanager.Table]*sql.Stmt

	lastIDQuery      *sql.Stmt
	latePVCPodMount  *sql.Stmt // Used if the PVC was created after the pod.
	fallbackPodMount *sql.Stmt // Used if we haven't seen the PVC yet.
	addPVCPodMount   *sql.Stmt

	clearBadPodMount *sql.Stmt // Cleans up incorrect pod mounts from latePVCPodMount

	updateRVQuery *sql.Stmt
	getRVQuery    *sql.Stmt
}

// Destroy tears down all structures associated with a mySQLManager object,
// closing the database connection and any prepared statements.
func (dbm *mySQLManager) Destroy() {
	// Note that these functions do the right thing and return if the
	// relevant statements haven't been initialized.
	dbm.destroyInsertStatements()
	dbm.destroyDeleteStatements()
	dbm.destroyBindStatements()

	dbm.destroyExistenceQueries()
	dbm.destroyRVQueries()

	if dbm.lastIDQuery != nil {
		dbm.lastIDQuery.Close()
	}
	if dbm.latePVCPodMount != nil {
		dbm.latePVCPodMount.Close()
	}
	if dbm.fallbackPodMount != nil {
		dbm.fallbackPodMount.Close()
	}
	if dbm.addPVCPodMount != nil {
		dbm.addPVCPodMount.Close()
	}
	if dbm.clearBadPodMount != nil {
		dbm.clearBadPodMount.Close()
	}
	if dbm.updateRVQuery != nil {
		dbm.updateRVQuery.Close()
	}
	if dbm.getRVQuery != nil {
		dbm.getRVQuery.Close()
	}
	dbm.db.Close()
}

// runTxActual wraps the code in txFunc with a database transaction.
// Inspired from code here:
// http://stackoverflow.com/a/23502629/145587
func (m *mySQLManager) runTxActual(txFunc func(tx *sql.Tx) error) (err error) {
	tx, err := m.db.Begin()
	if err != nil {
		log.Print("Unable to start transaction: ", err)
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			mysqlErr, ok := err.(*mysql.MySQLError)
			// Don't error on duplicates; it's likely that we're processing
			// events that we've already seen due to needing to restart.
			// E.g., if the resource version is stale, we need to reinitialize
			// fully; it's easier to just ignore duplicate errors than it is
			// to catch and deal with them.
			if ok && mysqlErr.Number == ErrDuplicateKey {
				// TODO:  Either get better error handling or consider removing
				// this.
				log.Print("Inserted duplicate key")
				err = nil
				return
			}
			return
		}
		err = tx.Commit()
	}()
	err = txFunc(tx)
	return
}

// runTx serves as a wrapper around runTxActual to make it easier to retry
// the function if a deadlock results.
func (m *mySQLManager) runTx(txFunc func(tx *sql.Tx) error) (err error) {
	success := false
	for !success && err == nil {
		err := m.runTxActual(txFunc)
		if err == nil {
			success = true
		} else if mySQLErr, ok := err.(*mysql.MySQLError); ok && mySQLErr.Number == deadlockErrNo {
			// If the error is a deadlock, we need to retry
			// TODO:  Insert max number of attempts?
			err = nil
			// TODO:  Is this sleep really necessary?
			time.Sleep(time.Millisecond * 50)
		}
	}
	return
}

// doTxStatementCheckRows executes a prepared statement stored in one of
// mySQLManager's statement maps, returning the number of rows affected along
// with any errors.
func (dbm *mySQLManager) doTxStatementCheckRows(tx *sql.Tx, query_type string,
	table dbmanager.Table,
	stmtMap map[dbmanager.Table]*sql.Stmt,
	args ...interface{}) (int64, error) {

	var result sql.Result
	var err error

	result, err = tx.Stmt(stmtMap[table]).Exec(args...)
	if err != nil {
		log.Printf("Unable to execute %s query into table %s with args "+
			"%s: %s\n", query_type, table, args, err)
		return -1, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return -1, fmt.Errorf("Unable to retrieve number of rows affected for "+
			"%s query into table %s with args %s:  %s\n", query_type,
			table, args, err)
	}
	return rows, err
}

// doTxStatement wraps doTxStatementCheckRows, ignoring the rows returned.
func (m *mySQLManager) doTxStatement(tx *sql.Tx, query_type string,
	table dbmanager.Table,
	stmtMap map[dbmanager.Table]*sql.Stmt,
	args ...interface{}) error {

	_, err := m.doTxStatementCheckRows(tx, query_type, table, stmtMap,
		args...)
	return err
}

func (m *mySQLManager) ValidateConnection() error {
	var err error
	for tries := 0; tries < maxTries; tries++ {
		time.Sleep(time.Duration(tries) * time.Second)
		if err = m.db.Ping(); err == nil {
			return nil
		}
	}
	return err
}

// NewParams returns a DBManager instance backed by a MySQL database, using
// the supplied parameter string to modify the connection, as described here:
// https://github.com/go-sql-driver/mysql
// The connnection is established with the supplied username and password
// to the database specified in dbName at the IP address in dbAddr.
func NewParams(username, password, dbAddr, dbName string,
	params string) dbmanager.DBManager {

	m := new(mySQLManager)
	connection := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", username, password,
		dbAddr, dbName)
	if params != "" {
		connection = fmt.Sprintf("%s?%s", connection, params)
	}
	db, err := sql.Open("mysql", connection)
	if err != nil {
		log.Fatalf("Unable to connect to %s using connection string %s:  %s\n",
			dbAddr, connection, err)
	}
	m.db = db
	err = m.ValidateConnection()
	if err != nil {
		log.Fatal("Unable to connect to database.")
	}

	// Initialization methods.  These have been pulled out to make the
	// constructor more legible, but could be inlined at a future date.
	err = m.initInsertStatements()
	if err != nil {
		log.Fatal("Unable to create insert statements")
		goto cleanup // This is here in case we change Fatal to Print above.
	}
	err = m.initDeleteStatements()
	if err != nil {
		log.Fatal("Unable to create delete statements")
		goto cleanup // This is here in case we change Fatal to Print above.
	}

	err = m.initBindStatements()
	if err != nil {
		log.Fatal("Unable to create bind statements")
		goto cleanup
	}

	err = m.initUpdateStatements()
	if err != nil {
		log.Fatal("Unable to create update statements")
		goto cleanup
	}

	err = m.initExistenceQueries()
	if err != nil {
		log.Fatal("Unable to create existence queries")
		goto cleanup
	}

	err = m.initRVQueries()
	if err != nil {
		log.Fatal("Unable to create resource version queries")
	}

	m.lastIDQuery, err = m.db.Prepare("SELECT LAST_INSERT_ID()")
	if err != nil {
		log.Fatal("Unable to prepare statement to get the most recent " +
			"autoincrement ID.")
		goto cleanup
	}

	return m

cleanup:
	m.Destroy()
	return nil
}

// NewForDB wraps NewForParams, using a default set of connection parameters
// ("parseTime=true")
func NewForDB(username, password, dbAddr, dbName string) dbmanager.DBManager {
	return NewParams(username, password, dbAddr, dbName, "parseTime=true")
}

// New wraps NewForDB, creating a connection to the kubevoltracker database.
func New(username, password, dbAddr string) dbmanager.DBManager {
	return NewForDB(username, password, dbAddr, "kubevoltracker")
}
