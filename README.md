Kubernetes Volume Tracker
------------------------

The persistent volume and persistent volume claim interface provides a powerful
storage abstraction for Kubernetes users:  applications can obtain storage
automatically based on their needs, without having to go through a cumbersome
administrative process.  On the other hand, the automated nature of this
process leaves storage administrators with very little idea of who is using
what storage, for what purpose.  To remedy this situation, we provide the
Kubernetes Volume Tracker.  

The Volume Tracker listens for pod, PV, and PVC events from the API Server
and uses them to infer the current state of storage usage in the cluster.  It
then persists this state in a backing MySQL database, which administrators
can then query to determine both how storage is currently being used and
how it has been used over time.

More information on the motivation, design, and future plans of the Usage
Tracker is outlined in `documentation/proposal.md`.  The remainder of this
document will explain how to configure, build, and run the Volume Tracker.

Environment Variables
=====================

Initializing the Volume Tracker database and running the main binary requires
that several environment variables be set.  These are as follows:

* `MYSQL_IP`:  The IP address of the backing MYSQL database.
* `KUBERNETES_MASTER`:  The IP address and port of the Kubernetes API server. 

In addition, the test suite requires several additional environment variables.
These are:

* `FILER_IP`:  The IP address of an NFS server.
* `EXPORT_ROOT`:  The root export of that NFS server.  The test suite will
  routinely clobber the contents of this export, so nothing important should be
  located here.
* `LOCAL_EXPORT_ROOT`:  The path to a local directory (on the machine running
  the test) where the NFS server's root export is mounted.  This is used to
  create subdirectories that different PVs mount.
* `TARGET_PORTAL`:  The target portal IP address and port of an ISCSI target
  with one or more LUNs configured for use during testing.
* `IQN`:  The IQN of the ISCSI target.

Installation
============

**Build**

The Volume Tracker uses [glide](https://github.com/Masterminds/glide) to
fetch its dependencies prior to building.  `make glide` will install Glide
and use it to retrieve the necessary dependencies.  Once this is done, you
can build the project by running `go build` in the project root or by
using `make`, which builds the project in a Docker container and copies
the binary into the `docker_image` directory.

**Database Setup**

In addition to the main binary, the Volume Tracker also requires a running MySQL
database.  The `kubernetes-yaml` subdirectory provides pod and service
definitions (`mysql.yaml` and `mysql-service.yaml`, respectively) for such a
database, if one is not already present.  The default
username and password is root/root, though this can be changed in the pod
definition.  The pod depends on the PVC defined in `kubevoltracker-pvc.yaml`,
though this dependency can be changed to use a different PVC or an external
volume.

To initialize the database, run
`create_db.sh <mysql-ip-address> <mysql-username> <mysql-password>`, using the
database username and password.  This creates a database and initializes the
tables that the Volume Tracker needs.

**Docker Image**

To facilitate deploying the Volume Tracker in a container, the `docker_image`
subdirectory includes a Dockerfile that builds a Docker image containing
the Volume Tracker.  This requires the `kubevoltracker` binary to be present
in the directory; to do so, either run `make`, which will automatically
copy the binary, or build the project and copy the binary into the directory
manually.  To create and run the image successfully, three build arguments
can be provided:

* `KUBERNETES_MASTER_IP`:  The IP address of the Kubernetes API server.
* `KUBERNETES_MASTER_PORT`:  The port number of the Kubernetes API server.
* `MYSQL_IP`:  The IP address of the backing MySQL database.

These arguments are used to prepopulate the image's environment variables, 
allowing it to be run without additional arguments.  Alternatively, the
environment variables can be specified at run time, using command line arguments
or, if running the image in Kubernetes, in the pod definition, described below.

**Kubernetes Pod**

For running the Volume Tracker in Kubernetes, we have provided a sample
pod template and PVC definition (`kubevoltracker.yaml.templ` and
`kubevoltracker-pvc.yaml`) 
in the `kubernetes-yaml` subdirectory.  The PVC definition requires a
ReadWriteOnce PV with at least 1 GB of storage. To create the pod, the image for
the kubevoltracker container must be modified to point to a local registry
containing the kubevoltracker image, and it may be necessary to change the
port of the API server, depending on the Kubernetes configuration.  The IP 
ddress of the API server is discovered through the Service API and thus should
not need to change, although specifying the IP address directly instead may
improve performance and stability.

The pod includes containers for both `kubevoltracker` and the MySQL database.
It does no initialization of the database, however, so it may be necessary
to run the `create_db.sh` script against the standalone MySQL pod initially.
Once this is done, the standalone pod can be stopped and the Volume Tracker
pod can be started; as they use the same PVC, the database will be correctly
initialized.  `create_db.sh` can also be run against the kubevoltracker pod
directly, although it may take some time for the pod's initial errored state
to clear.

Running
=======

Once the database has been started and configured and the appropriate
environment variables have been set, the Volume Tracker can be launched with

`kubevoltracker <-u username> <-p password>`

where username and password are the database username and password.  If omitted,
these parameters default to `root` and `root`.  Once started, the Volume Tracker
will run until terminated by the user.

If the Volume Tracker crashes or is stopped, it will query the API server for
any changes it may have missed once it restarts.  Assuming that it is restarted
promptly, it should not miss any events in the cluster.  However, if it is
down for an extended period of time, it will make a best effort attempt
to reconstruct the cluster state based on the objects currently present in the
cluster.

Querying
========

We provide several sample queries under the `queries` subdirectory.  Although
users can run the .sql queries directly, we recommend using the shell scripts
where provided.  Ad hoc queries against the database are also possible; we
describe the schema in `documentation/proposal.md`.

Load Test
=========

For basic stress testing, we provide a simple load generator under the
`load_test` subdirectory.  This is written in Python 2.7 and has a dependency
on `enum34` (a backport of the Python 3.4 enum module), which can be installed
with `pip install enum34`.  For usage instructions, run
`python load_generator.py --help`.
