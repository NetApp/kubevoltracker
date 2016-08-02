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
	"testing"

	"k8s.io/kubernetes/pkg/api"
)

func TestUpdatePVC(t *testing.T) {
	if fileName, err := UpdatePVC("default-apply", namespace, 2*1024*1024,
		[]api.PersistentVolumeAccessMode{api.ReadWriteOnce},
	); err != nil {
		t.Errorf("Unable to edit PVC at %s:  %s", fileName, err)
	}
}

func TestUpdatePV(t *testing.T) {
	exportDir := "update-subdir"
	RMExportDir(exportDir)
	if fileName, err := UpdatePV("default-apply", 2*1024*1024,
		[]api.PersistentVolumeAccessMode{api.ReadWriteMany, api.ReadOnlyMany},
	); err != nil {
		t.Errorf("Unable to edit PV at %s: %s", fileName, err)
	}
	if fileName, err := UpdatePVAtExport("default-apply", exportDir,
		4*1024*1024,
		[]api.PersistentVolumeAccessMode{api.ReadWriteOnce, api.ReadOnlyMany},
	); err != nil {
		t.Errorf("Unable to update PV for new export at %s: %s", fileName,
			err)
	}
	validateExportPath(t, exportDir)
	RMExportDir(exportDir)
}

func TestUpdateISCSIPV(t *testing.T) {
	// Initialize the PV if it didn't already exist; ignore any errors.
	CreateISCSIPV("default-iscsi", 1024*1024, 0)
	if fileName, err := UpdateISCSIPV("default-iscsi", 2*1024*1024, 0); err != nil {
		t.Errorf("Unable to update ISCSI PV at %s: %s", fileName, err)
	}
}
