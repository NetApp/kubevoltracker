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
	"errors"
	"fmt"
	"io"
	"log"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/netapp/kubevoltracker/dbmanager"
	"github.com/netapp/kubevoltracker/resources"
)

// Watcher stores the relevant data needed to watch the API server for changes
// to volume usage.
type Watcher struct {
	dbm dbmanager.DBManager

	client    *APIClient
	namespace string
	// TODO:  Figure out how this interfaces with the rest of the system.

	stopChannels map[resources.ResourceType]chan<- struct{}
}

type eventHandler func(EventType, resources.Resource, string)

// getRV returns the latest known resource for the given type, if it exists.
// If initialize is true, it returns an empty string.
func (w *Watcher) getRV(resource resources.ResourceType, initialize bool) (rv string) {
	var ns string

	if initialize {
		return ""
	}
	if resource == resources.PVs {
		ns = resources.PVNamespace
	} else {
		ns = w.namespace
	}
	rv = w.dbm.GetRV(resource, ns)
	log.Printf("Returning RV %s for %s in namespace %s", rv, resource,
		w.namespace)
	return
}

// handlePods communicates Pod events down to the back-end DBManager.
func (w *Watcher) handlePods(eventType EventType, r resources.Resource, json string) {
	p := r.(*resources.PodResource)
	uid := p.GetUID()
	switch eventType {
	case Added:
		pvcNames := make(map[string]string)
		for _, vol := range p.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				pvcNames[vol.Name] = vol.PersistentVolumeClaim.ClaimName
			}
		}
		containers := make([]resources.ContainerDesc, len(p.Spec.Containers))
		for i, container := range p.Spec.Containers {
			containerPVCMounts := make([]resources.VolumeMount, 0,
				len(container.VolumeMounts))
			for _, mount := range container.VolumeMounts {
				if pvcName, ok := pvcNames[mount.Name]; ok {
					containerPVCMounts = append(containerPVCMounts,
						resources.VolumeMount{Name: pvcName,
							ReadOnly: mount.ReadOnly})
				}
			}
			containers[i] = resources.ContainerDesc{
				Name:      container.Name,
				Image:     container.Image,
				Command:   GetCommandString(container.Command),
				PVCMounts: containerPVCMounts,
			}
		}
		w.dbm.InsertPod(uid, p.Name, p.CreationTimestamp, p.Namespace,
			containers, json, w.namespace, p.ResourceVersion)
	case Modified:
		// TODO:  Special handling here?  At least store the RV?
	case Error:
		// TODO:  Special handling here?
	case Deleted:
		w.dbm.DeletePod(uid, *p.DeletionTimestamp, w.namespace,
			p.ResourceVersion)
	}
}

// handlePods communicates PV events down to the back-end DBManager.
func (w *Watcher) handlePVs(eventType EventType, r resources.Resource,
	json string) {

	p := r.(*resources.PVResource)
	uid := p.GetUID()
	switch eventType {
	case Added:
		var (
			backend   dbmanager.Table
			backendID int
		)
		if p.Spec.NFS != nil {
			backendID = w.dbm.InsertNFS(p.Spec.NFS.Server, p.Spec.NFS.Path)
			backend = dbmanager.NFS
		} else if p.Spec.ISCSI != nil {
			backendID = w.dbm.InsertISCSI(p.Spec.ISCSI.TargetPortal,
				p.Spec.ISCSI.IQN, int(p.Spec.ISCSI.Lun), p.Spec.ISCSI.FSType)
			backend = dbmanager.ISCSI
		}
		storage := p.Spec.Capacity[api.ResourceStorage]
		w.dbm.InsertPV(p.UID, p.Name, p.CreationTimestamp, backendID,
			backend, (&storage).Value(), p.Spec.AccessModes, json,
			p.ResourceVersion)
		if p.Spec.ClaimRef != nil {
			w.dbm.BindPVC(p.UID, p.Spec.ClaimRef.UID, unversioned.Now(),
				p.ResourceVersion)
		}
	case Modified:
		if p.Status.Phase == api.VolumeBound {
			if p.Spec.ClaimRef == nil {
				log.Panic("ClaimRef is null when volume is bound; this is",
					"unexpected")
			}
			// TODO:  This is *REALLY* vulnerable to clock skew/processing
			// delays, but at the moment, Kubernetes doesn't give us a
			// timestamp for status changes.
			w.dbm.BindPVC(p.UID, p.Spec.ClaimRef.UID, unversioned.Now(),
				p.ResourceVersion)
		} else if p.Status.Phase == api.VolumeAvailable {
			var (
				backend   dbmanager.Table
				backendID int
			)
			if p.Spec.NFS != nil {
				backendID = w.dbm.InsertNFS(p.Spec.NFS.Server, p.Spec.NFS.Path)
				backend = dbmanager.NFS
			} else if p.Spec.ISCSI != nil {
				backendID = w.dbm.InsertISCSI(p.Spec.ISCSI.TargetPortal,
					p.Spec.ISCSI.IQN, int(p.Spec.ISCSI.Lun),
					p.Spec.ISCSI.FSType)
				backend = dbmanager.ISCSI
			}
			storage := p.Spec.Capacity[api.ResourceStorage]
			w.dbm.UpdatePV(p.UID, backendID, backend, (&storage).Value(),
				p.Spec.AccessModes, json, p.ResourceVersion)
		}
	case Error:
		// TODO:  Special handling here?
	case Deleted:
		if p.DeletionTimestamp != nil {
			w.dbm.DeletePV(uid, *p.DeletionTimestamp, p.ResourceVersion)
		} else {
			// This is less than ideal, but we don't have a choice.  See
			// warnings about clock skew in the comments for binding.
			w.dbm.DeletePV(uid, unversioned.Now(), p.ResourceVersion)
		}
	}
}

// handlePods communicates PVC events down to the back-end DBManager.
func (w *Watcher) handlePVCs(eventType EventType, r resources.Resource, json string) {
	p := r.(*resources.PVCResource)
	uid := p.GetUID()
	switch eventType {
	case Added:
		storage := p.Spec.Resources.Requests[api.ResourceStorage]
		// TODO:  Use p.Spec or p.Status?
		w.dbm.InsertPVC(p.UID, p.Name, p.CreationTimestamp, p.Namespace,
			(&storage).Value(), p.Spec.AccessModes,
			json, w.namespace, p.ResourceVersion)
	case Modified:
		// TODO:  Does anything need to happen here?  We currently manage
		// everything in the PV update, which is probably enough.
		if p.Status.Phase == api.ClaimPending {
			storage := p.Spec.Resources.Requests[api.ResourceStorage]
			w.dbm.UpdatePVC(p.UID, (&storage).Value(), p.Spec.AccessModes, json,
				w.namespace, p.ResourceVersion)
		}
	case Error:
		// TODO:  Special handling here?
	case Deleted:
		if p.DeletionTimestamp != nil {
			w.dbm.DeletePVC(uid, *p.DeletionTimestamp, w.namespace,
				p.ResourceVersion)
		} else {
			// This is less than ideal, but we don't have a choice.  See
			// warnings about clock skew in the comments for binding.
			w.dbm.DeletePVC(uid, unversioned.Now(), w.namespace,
				p.ResourceVersion)
		}
	}
}

// getHandler returns the appropriate handler for a given resource type
// (e.g., for resources.Pods, it returns w.handlePods).
func (w *Watcher) getHandler(resource resources.ResourceType) (eventHandler, error) {
	switch resource {
	case resources.Pods:
		return w.handlePods, nil
	case resources.PVs:
		return w.handlePVs, nil
	case resources.PVCs:
		return w.handlePVCs, nil
	default:
		return nil, fmt.Errorf("Unable to handle resource type %s\n",
			resource)
	}
}

// Watch monitors a resource, starting a goroutine that uses the APIClient
// to watch a given endpoint and then calling the appropriate handler method
// for the events that come in.
// Returns a channel used to signal the watch to stop.
func (w *Watcher) Watch(resource resources.ResourceType, initialize bool) error {
	var rv string

	if _, ok := w.stopChannels[resource]; ok {
		log.Printf("WARNING:  Attempting to watch resource %s multiple times."+
			"  Returning.\n", resource)
		return nil
	}

	stop := make(chan struct{})
	w.stopChannels[resource] = stop

	handler, err := w.getHandler(resource)
	if err != nil {
		return err
	}

	go func() {
		var event WatchEvent
		for {
			rv = w.getRV(resource, initialize)
			eventChan, done := w.client.Watch(resource, w.namespace, rv)
			for open := true; open; {
				select {
				case event = <-eventChan:
				case <-stop:
					fmt.Printf("Stopping watch on %s\n", resource)
					close(done)
					return
				}
				if event.Err == io.EOF {
					close(done)
					if event.JSONEvent != nil {
						status := event.JSONEvent.(unversioned.Status)
						if status.Code == StatusTooOld {
							// The resource version is expired, so just start
							// the watch without one.
							log.Print("Got expired RV; doing fresh " +
								"initialization.")
							initialize = true
						}
					}
					open = false
				} else if event.Err != nil {
					// TODO:  Insert some kind of backoff/retry in case
					// the error is transient?
					close(done)
					log.Fatal("Unable to read events: ", event.Err)
				} else {
					e := event.JSONEvent.(ResourceEvent)
					r := e.GetResource()
					if r.GetRV() == rv {
						log.Printf("Got duplicate RV %s for resource %s; "+
							"skipping.\n", rv, resource)
						continue
					}
					handler(e.GetType(), r, event.JSON)
					log.Println(e)
				}
			}
		}
	}()
	return nil
}

// Destroy stops all active goroutines associated with the Watcher, and
// calls Destroy on the backing DBManager.
func (w *Watcher) Destroy() {
	for _, ch := range w.stopChannels {
		close(ch)
	}
	w.dbm.Destroy()
}

// Stop stops a watch on the specified resource type.
func (w *Watcher) Stop(resource resources.ResourceType) error {
	if _, ok := w.stopChannels[resource]; !ok {
		return fmt.Errorf("Attempted to stop nonexistent watch on resource %s",
			resource)
	}
	close(w.stopChannels[resource])
	delete(w.stopChannels, resource)
	return nil
}

// NewWatcher returns a new Watcher object that monitors the specified
// namespace for the API Server located at masterIPPort, using the provided
// DBManager instance for backing storage.
func NewWatcher(namespace string, masterIPPort string,
	dbm dbmanager.DBManager) (*Watcher, error) {

	if dbm == nil {
		return nil, errors.New("dbm cannot be nil when calling NewWatcher.")
	}
	client, err := NewAPIClient(masterIPPort)
	if err != nil {
		return nil, err
	}
	return &Watcher{
		client:       client,
		dbm:          dbm,
		namespace:    namespace,
		stopChannels: make(map[resources.ResourceType]chan<- struct{}),
	}, nil
}
