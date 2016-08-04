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

import argparse
from enum import Enum
import os
import random
import subprocess
import sys
import time

from resources import *

NFS_DIR_ENV = "LOAD_TEST_NFS_DIR"
EXPORT_PATH_ENV = "LOAD_TEST_EXPORT_PATH"
NFS_SERVER_ENV = "LOAD_TEST_NFS_SERVER"

# Edit these contents to change the test parameters.  These are relatively
# unimportant, however.
MAX_SIZE = 10 #Maximum size of a PV or PVC
MEAN_VOLS_PER_CONTAINER = 1
MAX_VOLS_PER_CONTAINER = 5

PREFIXES = {
		ResourceType.pod:"load-test-pod",
		ResourceType.pv:"load-test-pv",
		ResourceType.pvc:"load-test-pvc",
		}

# Represents the probability for choosing a given action.
class ActionProbability(object):

	def __init__(self, action, p):
		self.action = action
		self.p = p

# Iterates over a list of ActionProbabilities, recalculating them so that they
# sum to 1.
def RecalculateProbabilities(prob_list):
	total_prob = sum([p.p for p in prob_list])
	for p in prob_list:
		p.p = p.p/total_prob	

# InitRNG generates a seed for the random number generator and prints it, so
# that runs can be repeated.
def InitRNG(seed):
	if not seed:
		# Taken from Stack Overflow:
		# http://stackoverflow.com/questions/5012560/how-to-query-seed-used-by-random-random
		seed = random.randint(0, sys.maxint)
	random.seed(seed)
	print("RNG seed:  %d" % seed)
	f = open("last_seed.txt", "w")
	f.write("%d" % seed)
	f.close()

class LoadGenerator(object):

	def __init__(self, maxPVs, maxPVCs, maxPods, nfs_dir, server, export_path):
		assert maxPVCs <= maxPVs, "Must not specify more PVCs than PVs."
		assert maxPods <= maxPVCs, "Must not specify more pods than PVCs."

		self.maxResources = {ResourceType.pod:maxPods, ResourceType.pv:maxPVs,
							 ResourceType.pvc:maxPVCs}
		# Each map in current maps the names of current members of that resource
		# to their instance; use this approach rather than an array to
		# facilitate dependent-resource lookups, which are needed to ensure we
		# don't delete in-use resources.
		self.current = {ResourceType.pod:{}, ResourceType.pv:{},
				ResourceType.pvc:{}}
		self.allResources = {ResourceType.pod:{}, ResourceType.pv:{},
				ResourceType.pvc:{}}
		self.created = {ResourceType.pod:0, ResourceType.pv:0,
				ResourceType.pvc:0}
		self.indices = {ResourceType.pod:0, ResourceType.pv:0,
				ResourceType.pvc:0}
		if nfs_dir:
			self.nfs_dir = nfs_dir
		else:
			self.nfs_dir = os.getenv(NFS_DIR_ENV)
		assert self.nfs_dir, ("Must specify NFS directory as a command-line "
			"option or via %s" % NFS_DIR_ENV)
		if export_path:
			self.export_path = export_path
		else:
			self.export_path = os.getenv(EXPORT_PATH_ENV)
		assert self.export_path, ("Must specify NFS export path as a "
			"command-line option or via %s" % EXPORT_PATH_ENV)
		if server:
			self.server = server
		else:
			self.server = os.getenv(NFS_SERVER_ENV)
		assert self.server, ("Must specify NFS server as a "
			"command-line option or via %s" % NFS_SERVER_ENV)

	# CreateResource does the necessary busywork to create a Kubernetes
	# resource.
	# Resource should be one of the resource enums, toReplace consists of a
	# series of tuples containing template variables and their replacements
	# for the resource definition, and constructor should be a closure that
	# takes resource name and returns an object representing the new resource.
	# Returns the resource it created or None if unsuccessful.
	def CreateResource(self, resourceType, constructor):
		if (len(self.current[resourceType]) == self.maxResources[resourceType]):
			return None
		# TODO:  Parameterize chances for name reuse?
		# The second half of the clause below is there to ensure that at least
		# one of these resources has been deleted, so a name will be available
		# for reuse
		print("self.indices[resourceType]=%d; self.maxResources[resourceType]"
				"=%d" % (self.indices[resourceType],
					self.maxResources[resourceType]))
		newName = (random.random < .75 or
				self.indices[resourceType] <= self.maxResources[resourceType])
		if newName:
			name = "%s-%d" % (PREFIXES[resourceType],
					self.indices[resourceType])
			self.indices[resourceType] += 1
		else:
			name = ""
			while name == "" or name in self.current[resourceType].keys():
				name = "%s-%d" % (PREFIXES[resourceType], random.randint(0,
					self.indices[resourceType]-1))
		newResource = constructor(name)
		newResource.CreateFromTemplate()
		self.current[resourceType][name] = newResource
		if newName:
			self.allResources[resourceType][name] = [newResource]
		else:
			self.allResources[resourceType][name].append(newResource)
		self.created[resourceType] += 1
		return newResource


	def CreatePV(self):
		size = random.randint(1, MAX_SIZE)
		def CreatePV(name):
			nfs_path = os.path.join(self.nfs_dir, name)
			export_path = os.path.join(self.export_path, name)
			return PV(name, nfs_path, self.server, export_path, size)
		return self.CreateResource(ResourceType.pv, CreatePV)

	def CreatePVC(self, max_size):
		# This should only be called *after* creating a PV; ensure that
		# the size is no larger than the created PV, so that this will bind.
		size = random.randint(1, max_size)
		return self.CreateResource(ResourceType.pvc,
				lambda name: PVC(name, size))

	# CreateVol and CreatePod always return True; this is so that we can reuse
	# the DoAction code for both deletion and creation.  Deletion can fail,
	# while, as implemented currently, creation can't.
	def CreateVol(self):
		pv = None
		while not pv:
			# PV might be none if we've maxed it out, so we may need to delete
			# a PV before we can create a new volume; this is OK.
			pv = self.CreatePV()
			if not pv:
				self.DeletePV()
		pvc = self.CreatePVC(pv.size)
		pv.Bind(pvc)
		return True

	def CreatePod(self):
		pvcCount = int(min(
							round(random.expovariate(MEAN_VOLS_PER_CONTAINER)),
							MAX_VOLS_PER_CONTAINER,
							len(self.current[ResourceType.pvc]))
						)
		pvcs = random.sample(self.current[ResourceType.pvc].values(), pvcCount)
		newPod = self.CreateResource(ResourceType.pod,
				lambda name: Pod(name, pvcs))
		#return newPod
		return True

	# DeleteResource iterates through the list of resources of the provided type
	# in random order and deletes the first resource that can safely be deleted
	# (i.e., that is not currently in use).  If none can, it returns false; it
	# returns true if it successfully deletes a resource.
	def DeleteResource(self, resourceType):
		print("Deleting %s" % resourceType)
		toDelete = None
		targets = self.current[resourceType].keys()
		random.shuffle(targets)
		while (not toDelete and len(targets) > 0):
			toDelete = targets.pop()
			if not self.current[resourceType][toDelete].CanDelete():
				print("Unable to delete %s" % toDelete)
				toDelete = None
		if not toDelete:
			print("Returning without deleting")
			return False
		resource = self.current[resourceType][toDelete]
		del self.current[resourceType][toDelete]
		resource.Delete()
		print("Deleted %s" % toDelete)
		return True

	# DeletePV is a wrapper function for DeleteResource that deletes a PV
	def DeletePV(self):
		return self.DeleteResource(ResourceType.pv)

	# DeletePVC is a wrapper function for DeleteResource that deletes a PVC
	def DeletePVC(self):
		return self.DeleteResource(ResourceType.pvc)

	# DeletePod is a wrapper function for DeleteResource that deletes a Pod
	def DeletePod(self):
		return self.DeleteResource(ResourceType.pod)

	# GetCreateProbability returns the probability that the experiment
	# will create the given resource type, when it has chosen to create.
	# The likelihood of creating a pod should be proportional to the number
	# of PVCs in the cluster and vice versa.  If there are none of either,
	# create a PV.
	def GetCreateProbability(self, resourceType):
		# Calculate the probability for a Pod
		total = (len(self.current[ResourceType.pod]) +
				len(self.current[ResourceType.pvc]))
		if total == 0:
			print("Returning 0 for pod create probability")
			ret = 0
		else:
			ret = len(self.current[ResourceType.pvc])/float(total)
			print("Pod create probability:  %f" % ret)
		if resourceType == ResourceType.pod:
			return ret
		return 1-ret

	# GetDeleteProbability returns the probability that the experiment will
	# delete a resource of the given type when it has chosen to delete.
	# Weight the probability for a given resource based on its proportion
	# of the total resources.
	def GetDeleteProbability(self, resourceType):
		totalResources = sum([len(m) for m in self.current.values()])
		assert totalResources, ("Deletion should not be possible with zero "
				"resources in the cluster.")
		return len(self.current[resourceType])/float(totalResources)

	# DoAction chooses between creation or deletion of a resource, based
	# on the amount of resources in the cluster, and then, based on the action
	# chosen, decides on a resource type.  As deletion of bound resources is not	# possible, deleting may require multiple attempts.  Finally, while PVs
	# and PVCs are always created together to ensure that bindings happen in
	# a predictable fashion, PVCs can be deleted independently of their
	# underlying PV (PVs are never rebound, however).
	def DoAction(self):
		print("Starting new action")
		choices = []
		createActions = (
				(ResourceType.pod, self.CreatePod),
				(ResourceType.pvc, self.CreateVol)
				)
		deleteActions = (
				(ResourceType.pod, self.DeletePod),
				(ResourceType.pvc, self.DeletePVC),
				(ResourceType.pv, self.DeletePV)
				)
		# Calculate the probability of deletion as directly proportional to
		# the ratio of the total number of resources to the maximum amount
		# of resources that can exist at one time.
		deleteProbability = (sum([len(m) for r, m in self.current.items()])/
				float(sum(self.maxResources.values())))
		if (random.random() < deleteProbability):
			for resourceType, func in deleteActions:
				if (len(self.current[resourceType]) > 0):
					choices.append(ActionProbability(func,
						self.GetDeleteProbability(resourceType)))
			print("Deleting.  Pods:  %d; PVCs: %d; PVs: %d" % (
				len(self.current[ResourceType.pod]),
				len(self.current[ResourceType.pvc]),
				len(self.current[ResourceType.pv]),
				))
		else:
			for resourceType, func in createActions:
				if (len(self.current[resourceType]) <
					self.maxResources[resourceType]):
					choices.append(ActionProbability(func,
						self.GetCreateProbability(resourceType)))
		success = False
		while len(choices) > 0 and not success:
			print("Length of choices:  %d" % len(choices))
			RecalculateProbabilities(choices)
			n = random.random()
			cumulativeP = 0
			action = None
			for i in range(len(choices)):
				if choices[i].p+cumulativeP > n:
					action = choices[i].action
					del choices[i]
					break
				cumulativeP += choices[i].p
			success = action()

	# Run runs a load test for the specified duration.
	def Run(self, duration):
		start_time = time.time()
		while (time.time() - start_time) < duration:
			action_start = time.time()
			self.DoAction()
			time.sleep(max(0, 0.5-(time.time()-action_start)))

if __name__=="__main__":
	parser = argparse.ArgumentParser(description="Load generator that creates "
		"a series of PVs, PVCs, and pods in the cluster automatically.  For "
		"testing the usage tracker.")
	parser.add_argument("max_pvs", type=int, help="Maximum number of PVs to "
		"create.")
	parser.add_argument("max_pvcs", type=int, help="Maximum number of PVCs to "
		"create.")
	parser.add_argument("max_pods", type=int, help="Maximum number of pods to "
		"create.")
	parser.add_argument("-t", "--time", type=int, help="Seconds to run the "
		"test.", default = 30)
	parser.add_argument("-n", "--nfs_dir", help="Local directory where the NFS "
		"share for the load test is mounted.  Can also specify with %s as an"
		" environment variable" % NFS_DIR_ENV)
	parser.add_argument("-s", "--server", help="IP address of the NFS server "
		"from which the load test share is exported.  Can also specify with %s"
		" as an environment variable" % NFS_SERVER_ENV)
	parser.add_argument("-S", "--seed", type=int, help="Seed for the RNG.  "
			"Will be automatically determined if unspecified.")
	parser.add_argument("-e", "--export_path", help="Base export path for "
			"NFS PVs.  Can also specify with %s as an environment variable"
			% EXPORT_PATH_ENV) 
	args = parser.parse_args()
	InitPaths()
	ClearExistingResources()
	InitRNG(args.seed)
	lg = LoadGenerator(args.max_pvs, args.max_pvcs, args.max_pods, args.nfs_dir,
				args.server, args.export_path)
	lg.Run(args.time)
