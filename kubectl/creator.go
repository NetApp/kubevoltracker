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
	"k8s.io/kubernetes/pkg/api"

	"github.com/netapp/kubevoltracker/resources"
)

// CreatePod creates a basic pod in Kubernetes with the specified containers.
func CreatePod(name, namespace string,
	containerDescs []resources.ContainerDesc) (string, error) {
	filename, err := writePod(name, namespace, containerDescs)
	if err != nil {
		return "", err
	}
	if err = RunKubectl(Create, filename, true); err != nil {
		return "", err
	}
	return filename, nil
}

// Creates a Kubernetes PVC with the specified attributes.
func CreatePVC(name, namespace string, mb int,
	accessModes []api.PersistentVolumeAccessMode) (string, error) {
	filename, err := writePVC(name, namespace, mb, accessModes)
	if err != nil {
		return "", err
	}
	if err = RunKubectl(Create, filename, true); err != nil {
		return "", err
	}
	return filename, nil
}

// CreatePVAtExport creates an NFS PV that mounts the specified export.
// Export should be a subdirectory of $EXPORT_ROOT; thus, if empty, it creates
// a PV mounting $EXPORT_ROOT.
func CreatePVAtExport(name, export string, mb int,
	accessModes []api.PersistentVolumeAccessMode) (string, error) {
	nfsVolumeSource := &api.NFSVolumeSource{
		Server: filerIP,
		Path:   GetExportPath(export),
	}

	makeExportDirectory(export)

	filename, err := writePV(name, nfsVolumeSource, nil, mb, accessModes)
	if err != nil {
		return "", err
	}
	if err = RunKubectl(Create, filename, true); err != nil {
		return "", err
	}
	return filename, nil
}

// CreatePV creates an NFS PV that mounts $EXPORT_ROOT.  Specifically, it calls
// CreatePVAtExport with "" as the export.
func CreatePV(name string, mb int,
	accessModes []api.PersistentVolumeAccessMode) (string, error) {
	return CreatePVAtExport(name, "", mb, accessModes)
}

// CreateISCSIPV creates an ISCSI PV with the given LUN and size, using
// the target portal and IQN specified in $TARGET_PORTAL and $IQN.
// All ISCSI PVs use the access modes specified in iscsiAccessModes.
func CreateISCSIPV(name string, mb int, lun int) (string, error) {
	iscsiVolumeSource := &api.ISCSIVolumeSource{
		TargetPortal: targetPortal,
		IQN:          iqn,
		Lun:          int32(lun),
		FSType:       iscsiFSType,
	}
	filename, err := writePV(name, nil, iscsiVolumeSource, mb,
		iscsiAccessModes)
	if err != nil {
		return "", err
	}
	if err = RunKubectl(Create, filename, false); err != nil {
		return "", err
	}
	return filename, nil
}
