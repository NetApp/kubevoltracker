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
)

// Note that, at the moment, we don't include edit functions for pods, since
// pods don't allow editing of fields that we care about.

// UpdatePVC updates the PVC with the specified name and namespace to have
// the size in MB and access modes provided.  This does no validation on whether
// the update should be possible.
func UpdatePVC(name, namespace string, mb int,
	accessModes []api.PersistentVolumeAccessMode) (string, error) {
	filename, err := writePVC(name, namespace, mb, accessModes)
	if err != nil {
		return "", err
	}
	if err = RunKubectl(Apply, filename, true); err != nil {
		return "", err
	}
	return filename, nil
}

// UpdatePVAtExport updates the PV with the provided name to point to
// the provided export and have the specified size in MB and access modes.
// As with CreatePVAtExport, export should be a subdirectory of $EXPORT_ROOT.
func UpdatePVAtExport(name, export string, mb int,
	accessModes []api.PersistentVolumeAccessMode) (string, error) {

	makeExportDirectory(export)

	nfsVolumeSource := &api.NFSVolumeSource{
		Server: filerIP,
		Path:   GetExportPath(export),
	}
	filename, err := writePV(name, nfsVolumeSource, nil, mb, accessModes)
	if err != nil {
		return "", err
	}
	if err = RunKubectl(Apply, filename, true); err != nil {
		return "", err
	}
	return filename, nil
}

// UpdatePV calls UpdatePVAtExport using "" as the export.
func UpdatePV(name string, mb int,
	accessModes []api.PersistentVolumeAccessMode) (string, error) {
	return UpdatePVAtExport(name, "", mb, accessModes)
}

// UpdateISCSIPV updates the ISCSI PV with the provided name to point to
// the provided LUN and have the specified size in MB.  Note that calling this
// for an NFS volume is undefined and should not be done.
func UpdateISCSIPV(name string, mb int, lun int) (string, error) {
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
	if err = RunKubectl(Apply, filename, false); err != nil {
		return "", err
	}
	return filename, nil
}
