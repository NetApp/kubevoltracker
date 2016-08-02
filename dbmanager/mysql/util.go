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

package mysql

import (
	"strings"

	"k8s.io/kubernetes/pkg/api"
)

func GetAccessModeString(accessModes []api.PersistentVolumeAccessMode) string {
	stringAccessModes := make([]string, len(accessModes))
	for i, m := range accessModes {
		stringAccessModes[i] = string(m)
	}
	return strings.Join(stringAccessModes, ",")
}
