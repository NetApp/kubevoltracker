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

from enum import Enum
import os
import shutil
import subprocess

# Enum used to represent the different types of resources that the cluster can
# create.
class ResourceType(Enum):
	pod = 1
	pv = 2
	pvc = 3

TEMPLATES = {
		ResourceType.pod:"pod-template.yaml",
		ResourceType.pv:"pv-template.yaml",
		ResourceType.pvc:"pvc-template.yaml",
		}
YAML_PATHS = {
		ResourceType.pod:"./pod_yaml",
		ResourceType.pv:"./pv_yaml",
		ResourceType.pvc:"./pvc_yaml",
		}

# Init paths creates directories to hold the yaml files produced for the
# different resource types.
def InitPaths():
	for resourceType, path in YAML_PATHS.items():
		if not os.path.exists(path):
			os.mkdir(path)

# Clear existing resources deletes all resources in the test namespace.
# It should be called prior to starting a run to ensure that no name collisions
# occur.
def ClearExistingResources():
	for r in ("pod", "pvc", "pv"):
		(output, err) = subprocess.Popen(
				"kubectl get %s --namespace=test-namespace" % r,
				stdout=subprocess.PIPE, stderr=subprocess.PIPE,
				shell=True).communicate()
		if err:
			sys.exit("Failed to get data for %s" % r)
		for name in [l.split()[0] if l else () for l in output.split("\n")]:
			if not name or name == 'NAME':
				continue
			subprocess.check_call("kubectl delete %s --grace-period=0 "
					"--namespace=test-namespace %s" % (r, name), shell=True)

# SubstituteTemplateVars runs sed on a template file.  For each
# (template, value) pair, it replaces template with value.
def SubstituteTemplateVars(filename, toReplace):
	for (template, var) in toReplace:
		subprocess.check_call('sed -i "s|%s|%s|g" %s' % (template, var,
			filename), shell=True)

# Base class representing a Kubernetes resource (pod, PV, or PVC).  Should
# be subclassed, but never instantiated.
class Resource(object):

	def __init__(self, name):
		self.name = name

	def GetFilename(self):
		return os.path.join(YAML_PATHS[self.GetResourceType()],
				"%s.yaml" % self.name)

	# Delete deletes the resource from Kubernetes.
	def Delete(self, params=""):
		cmd = 'kubectl delete -f %s' % self.GetFilename()
		if params:
			cmd = '%s %s' % (cmd, params)
		subprocess.check_call(cmd, shell=True)

	# PrepTemplate should take the necessary steps to initialize the template
	# for creation.
	def PrepTemplate(self, filename):
		assert False, "Subclasses of Resource must override PrepTemplate."

	# GetResourceType should return the ResourceType enum corresponding to
	# the subclass of Resource.
	def GetResourceType(self):
		assert False, "Subclasses of Resource must override GetResourceType."

	# CreateFromTemplate prepares a template for a resource and creates it
	# in Kubernetes.
	def CreateFromTemplate(self):
		resourceType = self.GetResourceType()
		filename = self.GetFilename()
		shutil.copyfile(TEMPLATES[resourceType], filename)
		self.PrepTemplate(filename)
		subprocess.check_call('kubectl create -f %s' % filename, shell=True)

	def CanDelete(self):
		assert False, "Subclasses of Resource must override CanDelete."
		return True

class Pod(Resource):

	# Pod needs only to take the PVCs that it requires and its name.
	# When instantiated, it calls Mount on each PVC passed in.
	def __init__(self, name, pvcs):
		self.pvcs = pvcs
		for pvc in self.pvcs:
			pvc.Mount()
		super(Pod, self).__init__(name)

	def GetResourceType(self):
		return ResourceType.pod

	def PrepTemplate(self, filename):
		SubstituteTemplateVars(filename, (("NAME", self.name),))
		to_append = []
		to_append.append("    volumeMounts:")
		for i in range(len(self.pvcs)):
			to_append.append("""    - name: nfs-%d
      mountPath: "/mnt/nfs-%d" """ % (i, i))
		i = 0
		to_append.append("  volumes:")
		for pvc in self.pvcs:
			to_append.append("""  - name: nfs-%d
    persistentVolumeClaim:
      claimName: %s """ % (i, pvc.name))
	  		i += 1
		f = open(filename, 'a')
		f.write("\n".join(to_append))
		f.close()

	# CanDelete always returns True for pods.
	def CanDelete(self):
		return True

	# Delete calls unmount on each PVC and then calls Delete in the superclass.
	def Delete(self):
		for pvc in self.pvcs:
			pvc.Unmount()
		super(Pod, self).Delete("--grace-period=0")

class PVC(Resource):

	# For PVCs, we only need to track the name and size.
	def __init__(self, name, size):
		self.size = size
		self.mounters = 0
		self.pv = None
		super(PVC, self).__init__(name)

	def GetResourceType(self):
		return ResourceType.pvc

	# Mount increments the number of mounters.
	def Mount(self):
		self.mounters += 1

	# Unmount decrements the number of mounters.
	def Unmount(self):
		self.mounters -= 1

	def PrepTemplate(self, filename):
		SubstituteTemplateVars(filename, (("NAME", self.name),
				("PVC_SIZE", "%d" % self.size)))

	# CanDelete only returns True if the PVC is not mounted.
	def CanDelete(self):
		return self.mounters == 0

	def Delete(self):
		assert self.pv, "Deleting unbound PVC; this should never happen."
		self.pv.Unbind()
		super(PVC, self).Delete()

class PV(Resource):

	# PVs only support NFS for now.  We care about their name, size, and mount
	# path.  export_path is the actual mount path, and nfs_path is a local
	# directory that, when created, will create the export_path on the server.
	def __init__(self, name, nfs_path, export_path, size):
		self.size = size
		self.nfs_path = nfs_path
		self.export_path = export_path
		self.pvc = None
		if not os.path.exists(nfs_path):
			os.mkdir(nfs_path)
		super(PV, self).__init__(name)

	def GetResourceType(self):
		return ResourceType.pv

	# Bind records that a PVC is bound to this PV.
	def Bind(self, pvc):
		self.pvc = pvc
		pvc.pv = self

	# Unbind clears the PVC stored in this PV.
	def Unbind(self):
		self.pvc = None

	# CanDelete only returns True if the PV is not bound.
	def CanDelete(self):
		return not self.pvc

	def PrepTemplate(self, filename):
		SubstituteTemplateVars(filename,
					(("NAME", self.name), ("PV_SIZE", "%d" % self.size),
				("PV_PATH", self.export_path)))

	def Delete(self):
		shutil.rmtree(self.nfs_path)
		super(PV, self).Delete()
