#!/usr/bin/env python
#  Copyright 2016 NetApp, Inc.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

import resources

import resources
import subprocess
import sys

subprocess.call("kubectl delete --namespace=test-namespace pod test "
	"--grace-period=0",
		shell=True)
subprocess.call("kubectl delete --namespace=test-namespace pod test-no-vol "
	"--grace-period=0",
		shell=True)
for name in ("foo", "bar"):
	subprocess.call("kubectl delete --namespace=test-namespace pv %s-pv" % name,
			shell=True)
	subprocess.call("kubectl delete --namespace=test-namespace pvc %s" % name,
			shell=True)
for name, size in (("foo", 1024), ("bar", 512)):
	pv = resources.PV("%s-pv" % name, "/kubetest/%spath" % name, size)
	pv.CreateFromTemplate()
	pvc = resources.PVC(name, size)
	pvc.CreateFromTemplate()
pod = resources.Pod("test", ("foo", "bar"))
pod.CreateFromTemplate()
no_vol_pod = resources.Pod("test-no-vol", ())
no_vol_pod.CreateFromTemplate()
