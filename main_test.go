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
	"os"
	"testing"

	"github.com/netapp/kubevoltracker/kubectl"
)

const watcherNS = "test"

func TestMain(m *testing.M) {
	kubectl.DeleteTestResources()
	v := m.Run()
	if v == 0 {
		// Only delete the test resources if everything succeeds; if there's
		// failures, leave the state around for diagnostic purposes.  Cleanup
		// can be accomplished by getting everything to run.
		kubectl.DeleteTestResources()
	}
	os.Exit(v)
}
