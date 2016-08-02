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

// Package testutils provides utility methods for testing interactions with
// a MySQL backend.
package testutils

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/netapp/kubevoltracker/resources"
)

var DB *sql.DB

// Type constants for use with ValidateResult.
// TODO:  There's probably a better way to handle this.
var (
	BoolType   = reflect.TypeOf(true)
	IntType    = reflect.TypeOf(0)
	Int64Type  = reflect.TypeOf(int64(0))
	StringType = reflect.TypeOf(sql.NullString{})
	TimeType   = reflect.TypeOf(mysql.NullTime{})
)

/* ValidateResult executes a SQL query and checks that its results match
   those provided in expected_values.  expected_types is an array with the
   reflect.Type values for expected_values; this allows nil values to be
   provided in expected_values.  Queries should only return a single row;
   mismatches between the length of expected_values and the number of returned
   values by the query will likely cause runtime errors.  Similarly, type
   failures will cause runtime errors.  Mismatched values will result
   in calls to t.Log with a description of the expected and actual values.
   Returns true if all values are correct and if only a single row was returned
   and false otherwise.
*/
func ValidateResult(t *testing.T, query string,
	expected_values []interface{}, expected_types []reflect.Type) bool {

	results := make([]interface{}, len(expected_values))
	result_ptrs := make([]interface{}, len(expected_values))
	for i := 0; i < len(results); i++ {
		results[i] = reflect.New(expected_types[i]).Interface()
		result_ptrs[i] = &results[i]
	}
	rows, err := DB.Query(query)
	if err != nil {
		log.Fatalf("Error in retrieval sql:  %s\nError:  %s\n", query, err)
	}
	rowsScanned := 0
	correct := true
	for rows.Next() {
		rowsScanned++
		// Designed for queries that only return a single row.
		if rowsScanned > 1 {
			t.Logf("Got extra row.\n")
			return false
		}
		err = rows.Scan(results...)
		if err != nil {
			log.Panic("Error when scanning results:  ", err)
		}
		for i := 0; i < len(results); i++ {
			switch local_type := results[i].(type) {
			case *bool:
				if *local_type != expected_values[i] {
					t.Logf("Value %d incorrect:\n\tExpected:  %t\n\tGot: %t",
						i, expected_values[i], *local_type)
					correct = false
				}
			case *int:
				if *local_type != expected_values[i] {
					t.Logf("Value %d incorrect:\n\tExpected:  %d\n\tGot: %d",
						i, expected_values[i], *local_type)
					correct = false
				}
			case *int64:
				if *local_type != expected_values[i] {
					t.Logf("Value %d incorrect:\n\tExpected:  %d\n\tGot: %d",
						i, expected_values[i], *local_type)
					correct = false
				}
			case *string:
				if *local_type != expected_values[i] {
					t.Logf("Value %d incorrect:\n\tExpected:  %s\n\tGot: %s",
						i, expected_values[i], *local_type)
					correct = false
				}
			case *sql.NullString:
				if expected_values[i] == nil {
					if (*local_type).Valid {
						t.Logf("Value %d incorrect:\n\tExpected nil\n\t"+
							"Got:  %v\n", i, (*local_type).String)
						correct = false
					}
					break
				}
				if (*local_type).String != expected_values[i] {
					t.Logf("Value %d incorrect:\n\tExpected:  %s.\n\tGot: %s.",
						i, expected_values[i], (*local_type).String)
					correct = false
				}
			case *mysql.NullTime:
				if expected_values[i] == nil {
					if (*local_type).Valid {
						t.Logf("Value %d incorrect:\n\tExpected:  nil\n\t"+
							"Got:  %v\n", i, (*local_type).Time)
						correct = false
					}
					break
				}
				// This requires fairly extensive parsing, since Timestamps
				// get converted to UTC and rounded to the microsecond, and time
				// equality doesn't account for this.
				// Note that the MySQL documentation claims that rounding should
				// occur instead of truncation, but it appears to be the reverse
				// in practice.
				// (http://dev.mysql.com/doc/refman/5.7/en/fractional-seconds.html
				if (*local_type).Time !=
					(expected_values[i].(time.Time)).UTC().Truncate(
						time.Microsecond) {
					t.Logf("Value %d incorrect.\n\tExpected:  %v\n\t"+
						"Got:  %v\n", i,
						(expected_values[i].(time.Time)).UTC().Truncate(
							time.Microsecond),
						(*local_type).Time,
					)
					correct = false
				}
			default:
				log.Fatalf("Unknown data type returned by query for value %d:"+
					"  %v\n", i, reflect.TypeOf(results[i]))
				correct = false
			}
		}
	}
	// We might scan 0 rows, in which case we're short
	if rowsScanned == 0 {
		t.Logf("Got no rows for query")
	}
	return correct && rowsScanned == 1
}

// ValidateResourceVersion checks that the stored resource version for the
// given namespace and resource is identical to that provided.
func ValidateResourceVersion(t *testing.T, resource resources.ResourceType,
	namespace, rv string) {

	correct := ValidateResult(t,
		"SELECT resource_version FROM resource_version WHERE resource LIKE '"+
			string(resource)+"' AND namespace LIKE '"+namespace+"'",
		[]interface{}{rv},
		[]reflect.Type{StringType},
	)
	if !correct {
		t.Errorf("Incorrect resource version for %s, %s\n", resource, namespace)
	}
}

// InitDB creates a database object shared across calls to the Validate
// functions.
func InitDB(dbName string) {
	var err error
	connection := fmt.Sprintf("root:root@tcp(%s:3306)/%s?parseTime=true",
		os.Getenv("MYSQL_IP"), dbName)
	DB, err = sql.Open("mysql", connection)
	if err != nil {
		log.Fatalf("Unable to connect to %s using connection string %s:  %s\n",
			os.Getenv("MYSQL_IP"), connection, err)
	}
}

// DestroyDB tears down the shared database created by InitDB.
func DestroyDB() {
	DB.Close()
}
