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

package kubectl

import (
	"os"
	"testing"
)

func validateExportPath(t *testing.T, subdir string) {
	st, err := os.Stat(GetLocalPath(subdir))
	if os.IsNotExist(err) {
		t.Error("Export directory not created for subdir.  Expected at ",
			GetLocalPath(subdir))
	} else if err != nil {
		t.Error("Unable to stat export dir: ", err)
	} else if !st.Mode().IsDir() {
		t.Error("Directory not created at export path ",
			GetLocalPath(subdir))
	}
}

func TestMain(m *testing.M) {
	DeleteTestResources()
	v := m.Run()
	if v == 0 {
		// As with main_test in the top level, only clean up the resources if
		// everything checks out.
		DeleteTestResources()
	}
	os.Exit(v)
}
