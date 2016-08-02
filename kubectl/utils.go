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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/netapp/kubevoltracker/resources"
)

type KubeAction string

const (
	Create KubeAction = "create"
	Delete KubeAction = "delete"
	Apply  KubeAction = "apply"
)

// GetContainerDescs is a utility function for CreatePod and UpdatePod that
// produces an array of resources.ContainerDesc, each of which contains
// the volumes specified in each top level entry of pvcsForContainers.
// Each container is named "busybox-%d", where %d is the index of the container
// in the returned array, and runs a busybox image with the command "sleep 3600"
func GetContainerDescs(
	pvcsForContainers [][]resources.VolumeMount,
) []resources.ContainerDesc {

	ret := make([]resources.ContainerDesc, len(pvcsForContainers))
	for i := 0; i < len(pvcsForContainers); i++ {
		cd := resources.ContainerDesc{
			Name:      fmt.Sprintf("busybox-%d", i),
			Image:     "busybox",
			Command:   "sleep 3600",
			PVCMounts: pvcsForContainers[i],
		}
		ret[i] = cd
	}
	return ret
}

// GetStorageValue converts a storage size in MB to an int64.
func GetStorageValue(size int) int64 {
	return int64(size) * 1024 * 1024
}

// makeExportDirectory creates a subdirectory in the local $EXPORT_ROOT mount
// that can be then be mounted in an NFS PV.
func makeExportDirectory(export string) {
	localDir := GetLocalPath(export)
	st, err := os.Stat(localDir)
	if os.IsNotExist(err) {
		if err = os.Mkdir(localDir, 0755); err != nil {
			log.Fatalf("Unable to create export dir at %s:  %s", localDir, err)
		}
	} else if err != nil {
		log.Fatalf("Unable to stat export directory at %s:  %s", localDir, err)
	} else if !st.Mode().IsDir() {
		log.Fatalf("Export target %s is not directory", localDir)
	}
}

// RunKubectl performs one of the kubectl actions (defined as a  KubeAction
// value) on the specified file.  If validate is false, it uses the
// --validate=false parameter.  If deleting, it appends the --grace-period=0
// flag so that tests do not have to wait for pods to terminate gracefully.
func RunKubectl(action KubeAction, file string, validate bool) error {
	args := []string{string(action), "-f", file}
	if action == Delete {
		args = append(args, "--grace-period=0")
	}
	if !validate {
		args = append(args, "--validate=false")
	}
	return exec.Command("kubectl", args...).Run()
}

// DeleteTestResources calls kubectl delete for every resource found in
// test-resources/.
// Note that, as implemented, baseDir depends on the package in which the
// initial test is invoked.  Thus, each package needs to do their own cleanup,
// as tests won't clean up resources from tests run in other packages.
func DeleteTestResources() {
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		log.Fatalf("Unable to read %s:  %s", baseDir, err)
	}
	for _, file := range files {
		err = exec.Command("kubectl", "delete",
			"--grace-period=0",
			"-f", path.Join(baseDir, file.Name())).Run()
	}
	fmt.Println("Cleared test resources.")
}

// RMExportDir removes a subdirectory of $EXPORT_ROOT.
func RMExportDir(dir string) {
	dir = GetLocalPath(dir)
	st, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		log.Fatalf("Unable to stat %s:  %s", dir, err)
	}
	if !st.Mode().IsDir() {
		log.Fatalf("%s is not a directory.", dir)
	}
	if err = os.RemoveAll(dir); err != nil {
		log.Fatalf("Unable to remove %s:  %s", dir, err)
	}
}
