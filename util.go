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

package main

import (
	"strings"
)

/* Needed because go doesn't include a maxint function */
func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetCommandString joins a string array with spaces.
func GetCommandString(command []string) string {
	return strings.Join(command, " ")
}
