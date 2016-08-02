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

package mock

import (
	"testing"
)

const (
	server1 = "127.0.0.1"
	server2 = "127.0.0.2"
	path1   = "/path1"
	path2   = "/path2"

	targetPortal1 = "127.0.0.1"
	targetPortal2 = "127.0.0.2"
	iqn1          = "iqn.2016-05.com.netapp:storage:example-iqn"
	iqn2          = "iqn.2016-05.com.netapp:storage:example-iqn2"
	lun1          = 0
	lun2          = 1
	fsType1       = "ext4"
	fsType2       = "btrfs"
)

func TestInsertNFS(t *testing.T) {
	m := New(false)
	nfsID := m.InsertNFS(server1, path1)
	newNFSID := m.InsertNFS(server1, path1)
	if nfsID != newNFSID {
		t.Errorf("Expected identical NFS ID.\n\tExpected: %d; got %d\n",
			nfsID, newNFSID)
	}
	newNFSID = m.InsertNFS(server1, path2)
	if nfsID == newNFSID {
		t.Error("Expected different NFS ID for different path.")
	}
	newNFSID = m.InsertNFS(server2, path1)
	if nfsID == newNFSID {
		t.Error("Expected different NFS ID for different server.")
	}
	newNFSID = m.InsertNFS(server2, path2)
	if nfsID == newNFSID {
		t.Error("Expected different NFS ID for different server and path.")
	}
}

func TestInsertISCSI(t *testing.T) {
	m := New(false)
	iscsiID := m.InsertISCSI(targetPortal1, iqn1, lun1, fsType1)
	newISCSIID := m.InsertISCSI(targetPortal1, iqn1, lun2, fsType2)
	if iscsiID == newISCSIID {
		t.Error("Expected different ISCSI ID for different lun, fs type.")
	}
	newISCSIID = m.InsertISCSI(targetPortal2, iqn2, lun1, fsType1)
	if iscsiID == newISCSIID {
		t.Error("Expected different ISCSI ID for different target portal, " +
			"iqn.")
	}
	newISCSIID = m.InsertISCSI(targetPortal1, iqn2, lun2, fsType1)
	if iscsiID == newISCSIID {
		t.Error("Expected different ISCSI ID for different iqn, lun")
	}
	newISCSIID = m.InsertISCSI(targetPortal1, iqn1, lun1, fsType1)
	if iscsiID != newISCSIID {
		t.Error("Expected duplicate ISCSI ID for duplicate lun data.")
	}
}
