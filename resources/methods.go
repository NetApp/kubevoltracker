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

package resources

import (
	"fmt"

	"k8s.io/kubernetes/pkg/types"
)

// Resource is used to wrap Kubernetes API elements to provide some degree
// of polymorphism for basic methods (getting the UID and resource version of
// an object, as well as its string representation).
type Resource interface {
	GetUID() types.UID
	GetRV() string
	String() string // Here for debugging purposes.
}

func (p *PodResource) GetUID() types.UID { return p.UID }
func (p *PVResource) GetUID() types.UID  { return p.UID }
func (p *PVCResource) GetUID() types.UID { return p.UID }

func (p *PodResource) GetRV() string { return p.ResourceVersion }
func (p *PVResource) GetRV() string  { return p.ResourceVersion }
func (p *PVCResource) GetRV() string { return p.ResourceVersion }

func (p *PodResource) String() string {
	ret := fmt.Sprintf("Pod %s, host %s, RV %s",
		p.Name,
		p.Status.HostIP,
		p.ResourceVersion)
	if p.Status.Phase != "" {
		ret += fmt.Sprintf(", phase %s", p.Status.Phase)
	}
	if p.Status.Message != "" {
		ret += fmt.Sprintf(", status %s", p.Status.Message)
	}
	if p.Status.Reason != "" {
		ret += fmt.Sprintf("\n\tReason:  %s", p.Status.Reason)
	}
	return ret
}

func (p *PVResource) String() string {
	return fmt.Sprintf("PV %s, RV %s",
		p.Name,
		p.ResourceVersion)
}
func (p *PVCResource) String() string {
	return fmt.Sprintf("PVC %s, RV %s\n",
		p.Name,
		p.ResourceVersion)
}
