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
	"log"
	"os"
	"testing"

	"github.com/netapp/kubevoltracker/kubectl"
	"github.com/netapp/kubevoltracker/resources"
)

func GetAPIClient() *APIClient {
	c, err := NewAPIClient(os.Getenv("KUBERNETES_MASTER"))
	if err != nil {
		log.Fatalf("Unable to create apiclient.  Aborting.  %s", err)
	}
	return c
}

func TestBadAPIClient(t *testing.T) {
	if _, err := NewAPIClient(""); err == nil {
		t.Error("Did not get an error return when no API server address or",
			"port was specified.")
	}
}

// This is a basic function, ensuring each channel creates channels correctly
// and returns the right objects.  There's not a lot more here that can be
// done without higher level semantics.
func TestWatches(t *testing.T) {
	var pvFileName, pvcFileName, podFileName string
	var err error

	kubectl.DeleteTestResources()
	a := GetAPIClient()
	pvChan, donePV := a.Watch("persistentvolumes", "", "")
	defer close(donePV)
	pvcChan, donePVC := a.Watch("persistentvolumeclaims", watcherNS,
		"")
	defer close(donePVC)
	podChan, donePod := a.Watch("pods", watcherNS, "")
	defer close(donePod)

	if pvFileName, err = kubectl.CreatePV("nfs", 1, defaultPVAccessModes); err != nil {
		t.Fatalf("Unable to create %s:  %s", pvFileName, err)
	}
	if pvcFileName, err = kubectl.CreatePVC(
		"nfs", watcherNS, 1, defaultPVCAccessModes,
	); err != nil {
		t.Fatalf("Unable to create %s:  %s", pvcFileName, err)
	}
	if podFileName, err = kubectl.CreatePod("nfs", watcherNS,
		kubectl.GetContainerDescs(
			[][]resources.VolumeMount{[]resources.VolumeMount{
				resources.VolumeMount{Name: "nfs", ReadOnly: false},
			},
			},
		),
	); err != nil {
		t.Fatalf("Unable to create %s:  %s", podFileName, err)
	}

	event := <-pvChan
	if event.Err != nil {
		t.Fatal("Unable to obtain basic watch:  ", event.Err)
	}
	_, ok := event.JSONEvent.(*PVEvent)
	if !ok {
		t.Fatal("Unable to cast to PVEvent:  ", event.JSONEvent)
	}

	event = <-pvcChan
	if event.Err != nil {
		t.Fatal("Unable to obtain basic watch:  ", event.Err)
	}
	_, ok = event.JSONEvent.(*PVCEvent)
	if !ok {
		t.Fatal("Unable to cast to PVCEvent:  ", event.JSONEvent)
	}

	event = <-podChan
	if event.Err != nil {
		t.Fatal("Unable to obtain basic watch:  ", event.Err)
	}
	_, ok = event.JSONEvent.(*PodEvent)
	if !ok {
		t.Fatal("Unable to cast to PVEvent:  ", event.JSONEvent)
	}
	return
}
