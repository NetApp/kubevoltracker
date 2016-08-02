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

	"github.com/netapp/kubevoltracker/resources"
)

const namespace = "test"

func TestCreatePod(t *testing.T) {
	if fileName, err := CreatePod("autocreate-pod", namespace,
		GetContainerDescs([][]resources.VolumeMount{
			[]resources.VolumeMount{resources.VolumeMount{"default", false}}}),
	); err != nil {
		t.Errorf("Unable to create pod at %s:  %s", fileName, err)
	}
	if fileName, err := CreatePod("novol-pod", namespace,
		GetContainerDescs([][]resources.VolumeMount{[]resources.VolumeMount{}}),
	); err != nil {
		t.Errorf("Unable to create no volume pod at %s:  %s", fileName, err)
	}
}

func TestCreateMultivolumePod(t *testing.T) {
	var (
		vol1              = "test-1"
		vol2              = "test-2"
		volumes           = []string{vol1, vol2}
		volumeAccessModes = []api.PersistentVolumeAccessMode{api.ReadWriteMany,
			api.ReadOnlyMany}
	)
	for i, volName := range volumes {
		if fileName, err := CreatePV(
			volName, i+1, volumeAccessModes,
		); err != nil {
			t.Fatalf("Unable to create PV %s at %s:  %s", volName, fileName,
				err)
		}
		if fileName, err := CreatePVC(
			volName, namespace, i+1, volumeAccessModes,
		); err != nil {
			t.Fatalf("Unable to create PVC %s at %s:  %s", volName, fileName,
				err)
		}
	}
	if fileName, err := CreatePod("multivolume", namespace,
		GetContainerDescs(
			[][]resources.VolumeMount{[]resources.VolumeMount{
				resources.VolumeMount{vol1, true}},
				[]resources.VolumeMount{resources.VolumeMount{vol1, false},
					resources.VolumeMount{vol2, true}}},
		),
	); err != nil {
		t.Errorf("Unable to create multivolume pod at %s:  %s", fileName, err)
	}
}

func TestCreatePVC(t *testing.T) {
	if fileName, err := CreatePVC("default", namespace, 1024*1024,
		[]api.PersistentVolumeAccessMode{api.ReadWriteOnce, api.ReadOnlyMany,
			api.ReadWriteMany},
	); err != nil {
		t.Errorf("Unable to create PVC at %s:  %s", fileName, err)
	}
}

func TestCreatePV(t *testing.T) {
	exportDir := "create-subdir"
	RMExportDir(exportDir)
	if fileName, err := CreatePV("default", 1024*1024,
		[]api.PersistentVolumeAccessMode{api.ReadWriteMany},
	); err != nil {
		t.Errorf("Unable to create PV at %s: %s", fileName, err)
	}
	if fileName, err := CreatePVAtExport("default-subdir", exportDir,
		1024*1024, []api.PersistentVolumeAccessMode{api.ReadWriteMany},
	); err != nil {
		t.Errorf("Unable to create PV at %s: %s", fileName, err)
	}
	validateExportPath(t, exportDir)
	RMExportDir(exportDir)
}

func TestCreateISCSIPV(t *testing.T) {
	if fileName, err := CreateISCSIPV("default-iscsi", 1024*1024, 0); err != nil {
		t.Errorf("Unable to create ISCSI PV at %s: %s", fileName, err)
	}
}
