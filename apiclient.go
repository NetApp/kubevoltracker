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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/netapp/kubevoltracker/resources"
)

// TODO:  Pull this out into an interface so it can't be constructed except with
//   NewAPIClient function defined below?
type APIClient struct {
	apiURL string
}

// WatchEvent represents an event received by the watcher.
type WatchEvent struct {
	JSONEvent interface{} // The parsed API object associated with the event.
	JSON      string      // The raw JSON string for the event.
	Err       error       // Any error code associated with the event.
}

/* Watch starts a goroutine that watches an API server endpoint.  It returns
   eventChan, a channel through which endpoint events are passed, and done,
   a channel used to signal that the watch should terminate.
   objectToWatch is optional.  Namespace can be empty if the watch is on
   a non-namespaced resource or if the watch is global (however, see NOTE).
   ResourceVersion can be empty for bootstrapping purposes, though it probably
   shouldn't be. */
// TODO:  This may be quite slow; consider using separate WatchEvent structs
//   for each potential type, at the very least.
// TODO:  If this ends up dropping events, create a buffered channel and
//   an array of WatchEvents to copy in.
// NOTE:  The outline of this function is derived from
//   https://github.com/golang/build/blob/master/kubernetes/client.go
// NOTE: resourceVersion isn't supposed to be consistent across multiple
//   namespaces; however, the global namespace appears to be a namespace in
//   and of itself.  If this isn't the case, I'm not sure how we should manage
//   this, since, as of Kubernetes 1.1, there is no way to list namespaces
//   that haven't been explicitly created (as opposed to implicitly created via
//   reference in a resource definition).
func (a *APIClient) Watch(objectToWatch resources.ResourceType,
	namespace string, resourceVersion string) (eventChan chan WatchEvent,
	done chan struct{}) {

	var namespaceComponent string
	var resourceVersionComponent string

	createEvent := resourceFactoryMap[objectToWatch]
	eventChan = make(chan WatchEvent)
	done = make(chan struct{})

	// Construct the URL
	if namespace != "" && objectToWatch != resources.PVs {
		namespaceComponent = fmt.Sprintf("namespaces/%s", namespace)
	} else {
		namespaceComponent = ""
	}
	if resourceVersion != "" {
		resourceVersionComponent = fmt.Sprintf("?resourceVersion=%s",
			resourceVersion)
	} else {
		resourceVersionComponent = ""
	}
	watchURL := fmt.Sprintf("%s/watch/%s/%s%s", a.apiURL,
		namespaceComponent, objectToWatch, resourceVersionComponent)
	fmt.Println("Watching URL ", watchURL)
	go func() {
		defer close(eventChan)
		resp, err := http.Get(watchURL)
		if err != nil {
			eventChan <- WatchEvent{Err: fmt.Errorf("Unable to "+
				"request watch for object %s at URL %s:  %s",
				objectToWatch, watchURL, err)}
			return
		}

		// Allow for external callers to close the watch.
		go func() {
			<-done
			resp.Body.Close()
		}()

		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)

		//TODO:  Add something here to safely clean up in case the
		// master thread exits?  Do we need to use /x/net/context here?
		// Might be necessary for breaking out of the HTTP request.
		for {
			startTime := time.Now()
			line, err := reader.ReadBytes('\n')
			endTime := time.Now()
			if err == io.EOF {
				eventChan <- WatchEvent{
					Err: err,
				}
				return
			}
			if err != nil {
				eventChan <- WatchEvent{
					Err: fmt.Errorf("Unable to read from "+
						"reader for %s after %s:  %s",
						objectToWatch,
						endTime.Sub(startTime),
						err)}
				return
			}
			event := createEvent()
			startTime = time.Now()
			err = json.Unmarshal(line, event)
			endTime = time.Now()
			if err != nil {
				var status StatusMessage
				if err = json.Unmarshal(line, &status); err == nil {
					eventChan <- WatchEvent{
						Err:       io.EOF,
						JSONEvent: status.Status,
						JSON:      string(line),
					}
				} else {
					eventChan <- WatchEvent{
						Err: fmt.Errorf("Unable to decode "+
							"json for %s:  %s\nJSON:\n%s",
							objectToWatch, err, line),
					}
				}
			}
			if event.GetType() == "" {
				var status unversioned.Status
				new_err := json.Unmarshal(line, &status)
				if new_err != nil {
					log.Print("Failed to parse status: ", new_err)
					log.Print("JSON: ", line)
					eventChan <- WatchEvent{
						Err: new_err,
					}
				} else {
					log.Print("Parsed status.")
					// Sending an EOF is a little sleazy, but it signals
					// that the handler needs to parse the results differently.
					// TODO:  Designate new error type?
					eventChan <- WatchEvent{
						Err:       io.EOF,
						JSONEvent: status,
						JSON:      string(line),
					}
				}
			} else {
				eventChan <- WatchEvent{JSONEvent: event, JSON: string(line)}
			}
		}
	}()
	return eventChan, done
}

// NewAPIClient returns a new client that can be used to place watches
// on the API server.  Takes the hostname/IP address and port of the Kubernetes
// API server; e.g. 192.168.1.1:8080
// NOTE:  Beyond 0 length checking, this does no URL validation.
func NewAPIClient(masterIpPort string) (*APIClient, error) {
	if len(masterIpPort) == 0 {
		return nil, errors.New("No master IP and port specified.  Unable to " +
			"create APIClient.")
	}
	return &APIClient{"http://" + masterIpPort + "/api/v1/"}, nil
}
