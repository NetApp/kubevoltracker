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

/*
   Package kubectl provides utilities for creating and updating Kubernetes
   resources.  It does so by writing YAML out to the test-resources subdirectory
   and calling kubectl create or kubectl apply as appropriate.  As this is
   intended for testing, all resources are basic and their details are
   relatively inflexible.

   All create and update functions return the filename of the YAML they have
   written, along with any error that may have occurred.
   TODO:  Communicate directly with the API server, rather than going through
   kubectl
*/
package kubectl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	k8sresource "k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/netapp/kubevoltracker/resources"
)

var (
	// These variables are needed to create NFS PVs
	baseDir    = path.Join(".", "test-resources")
	filerIP    = os.Getenv("FILER_IP")
	exportRoot = os.Getenv("EXPORT_ROOT")
	localRoot  = os.Getenv("LOCAL_EXPORT_ROOT")

	// These variables are needed to create ISCSI PVs
	targetPortal     = os.Getenv("TARGET_PORTAL")
	iqn              = os.Getenv("IQN")
	iscsiAccessModes = []api.PersistentVolumeAccessMode{api.ReadWriteOnce}
)

const (
	iscsiFSType = "ext4"
)

// init validates the variables that take their values from environment
// variables and performs some basic directory setup, as needed.
func init() {
	exit := false
	st, err := os.Stat(baseDir)
	if os.IsNotExist(err) {
		err = os.Mkdir(baseDir, 0755)
		if err != nil {
			log.Fatalf("Unable to create test resource dir %s:  %s", baseDir,
				err)
		}
	} else if err != nil {
		log.Fatalf("Unable to stat test resource dir %s:  %s", baseDir, err)
	} else if !st.IsDir() {
		log.Fatalf("%s must be a directory.  Exiting.", baseDir)
	}
	if filerIP == "" {
		log.Print("Must specify filer IP address in $FILER_IP")
		exit = true
	}
	if exportRoot == "" {
		log.Print("Must specify NFS root export in $EXPORT_ROOT")
		exit = true
	}
	if localRoot == "" {
		log.Print("Must specify local mount of NFS root export in ",
			"$LOCAL_EXPORT_ROOT")
		exit = true
	}
	if targetPortal == "" {
		log.Print("Must specify ISCSI target portal in $TARGET_PORTAL")
		exit = true
	}
	if iqn == "" {
		log.Print("Must specify ISCSI target IQN in $IQN")
		exit = true
	}
	if exit {
		os.Exit(1)
	}
}

// We use getters for the package variables above, since they should remain
// constant after initialization.
func GetFilerIP() string    { return filerIP }
func GetExportRoot() string { return exportRoot }
func GetExportPath(subdir string) string {
	return path.Join(exportRoot, subdir)
}
func GetLocalPath(subdir string) string {
	return path.Join(localRoot, subdir)
}
func GetTargetPortal() string { return targetPortal }
func GetIQN() string          { return iqn }
func GetFSType() string       { return iscsiFSType }
func GetISCSIAccessModes() []api.PersistentVolumeAccessMode {
	return iscsiAccessModes
}

// getFilename returns a suitable filename for a resource to be written.
func getFilename(name, resourceType string) string {
	return path.Join(baseDir, fmt.Sprintf("%s-%s.json", name, resourceType))
}

// writeResource writes a serialized JSON object to a file.
func writeResource(filename string, json []byte) error {
	return ioutil.WriteFile(filename, json, 0666)
}

// writePod creates an api.Pod object with the provided containers in the
// specified namespace and writes it to a yaml file.  It returns the filename
// and any errors that may have occurred.
func writePod(name, namespace string,
	containerDescs []resources.ContainerDesc) (string, error) {

	filename := getFilename(name, "pod")
	volumeMap := make(map[string]api.Volume)
	containers := make([]api.Container, len(containerDescs))
	for i, container := range containerDescs {
		volumeMounts := make([]api.VolumeMount, len(container.PVCMounts))
		for j, pvc := range container.PVCMounts {
			if _, ok := volumeMap[pvc.Name]; !ok {
				volumeMap[pvc.Name] = api.Volume{
					Name: pvc.Name,
					VolumeSource: api.VolumeSource{
						PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				}
			}
			volumeMounts[j] = api.VolumeMount{
				Name:      pvc.Name,
				MountPath: fmt.Sprintf("/mnt/nfs-%d", j),
				ReadOnly:  pvc.ReadOnly,
			}
		}
		containers[i] = api.Container{
			Name:         container.Name,
			Image:        container.Image,
			Command:      strings.Split(container.Command, " "),
			VolumeMounts: volumeMounts,
		}
	}
	volumes := make([]api.Volume, len(volumeMap))
	i := 0
	for _, vol := range volumeMap {
		volumes[i] = vol
		i++
	}
	p := api.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.PodSpec{
			Containers: containers,
			Volumes:    volumes,
		},
	}
	output, err := json.Marshal(p)
	if err != nil {
		log.Panic("Unable to marshal JSON: ", err)
	}
	err = writeResource(filename, output)
	return filename, err
}

// writePVC creates an api.PersistentVolumeClaimObject and writes it
// to a yaml file.  It returns the filename and any errors that may have
// occurred.
func writePVC(name, namespace string, mb int,
	accessModes []api.PersistentVolumeAccessMode) (string, error) {

	filename := getFilename(name, "pvc")
	size := GetStorageValue(mb)
	p := api.PersistentVolumeClaim{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: api.ResourceRequirements{
				Requests: map[api.ResourceName]k8sresource.Quantity{
					api.ResourceStorage: *k8sresource.NewQuantity(size,
						k8sresource.BinarySI),
				},
			},
		},
	}
	output, err := json.Marshal(p)
	if err != nil {
		log.Panic("Unable to marshal JSON: ", err)
	}
	err = writeResource(filename, output)
	return filename, err
}

// writePV creates an api.PersistentVolume backed by the provided volume
// source and writes it out to a yaml file.  Only one of nfsVolumeSource
// and iscsiVolumeSource should be non-nil.  It returns the filename
// and any errors.
func writePV(
	name string, nfsVolumeSource *api.NFSVolumeSource,
	iscsiVolumeSource *api.ISCSIVolumeSource, mb int,
	accessModes []api.PersistentVolumeAccessMode,
) (string, error) {

	filename := getFilename(name, "pv")
	size := GetStorageValue(mb)
	p := api.PersistentVolume{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: name,
		},
		Spec: api.PersistentVolumeSpec{
			AccessModes: accessModes,
			Capacity: api.ResourceList{
				api.ResourceStorage: *k8sresource.NewQuantity(size,
					k8sresource.BinarySI),
			},
			PersistentVolumeSource: api.PersistentVolumeSource{
				NFS:   nfsVolumeSource,
				ISCSI: iscsiVolumeSource,
			},
		},
	}
	output, err := json.Marshal(p)
	if err != nil {
		log.Panic("Unable to marshal JSON: ", err)
	}
	err = writeResource(filename, output)
	return filename, err
}
