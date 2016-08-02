CREATE TABLE IF NOT EXISTS nfs (
	id INT AUTO_INCREMENT PRIMARY KEY,
	ip_addr INT UNSIGNED NOT NULL, #NOTE:  Use inet_aton to store here.
	path VARCHAR(512) NOT NULL #TODO:  Handle overflow?
);
CREATE TABLE IF NOT EXISTS iscsi (
	id INT AUTO_INCREMENT PRIMARY KEY,
	target_portal VARCHAR(20) NOT NULL,
	iqn VARCHAR(128) NOT NULL,
	lun INT NOT NULL,
	fs_type VARCHAR(32)
);
#CREATE TABLE IF NOT EXISTS volume_source (
#	id INT AUTO_INCREMENT PRIMARY KEY,
#	type VARCHAR(256), #TODO:  Pull this out into an enum/separate table?
#	nfs_id INT REFERENCES nfs(id)
#   	#FOREIGN KEY nfs_id REFERENCES nfs
#	# TODO:  Add other foreign keys
#);
#NOTE THAT THE UID KEYS WILL PROBABLY NEED TO BE MADE AUTOINCREMENT.
CREATE TABLE IF NOT EXISTS pv (
	uid VARCHAR(64) PRIMARY KEY,
	name VARCHAR(256) NOT NULL,
	create_time TIMESTAMP(6) NOT NULL,
	delete_time TIMESTAMP(6),
	storage BIGINT NOT NULL,
	access_modes VARCHAR(128), -- This is more than we need, but it should work.
	json TEXT NOT NULL,
	nfs_id INT REFERENCES nfs(id),
	iscsi_id INT REFERENCES iscsi(id)
	# TODO:  Add other foreign keys
	);
CREATE TABLE IF NOT EXISTS pvc (
	uid VARCHAR(64) PRIMARY KEY, #TODO:  IS THIS LONG ENOUGH?
	name VARCHAR(256), -- Consider making name not nullable.
	-- Most fieldsMust be nullable for out-of-order binding
	create_time TIMESTAMP(6),
	delete_time TIMESTAMP(6),
	bind_time TIMESTAMP(6),
	namespace VARCHAR(256),
	storage BIGINT,
	access_modes VARCHAR(128), -- This is more than we need, but it should work.
	json TEXT,
	pv_uid VARCHAR(64) REFERENCES pv(uid)
	);
CREATE TABLE IF NOT EXISTS pod (
	uid VARCHAR(64) PRIMARY KEY,
	name VARCHAR(256) NOT NULL,
	create_time TIMESTAMP(6) NOT NULL,
	delete_time TIMESTAMP(6),
	namespace VARCHAR(256) NOT NULL,
	json TEXT NOT NULL
	);
-- We need to store the PVC name in the pod_mount to deal with races where the
-- pod is registered before the PVC.
CREATE TABLE IF NOT EXISTS pod_mount (
	pod_uid VARCHAR(64) REFERENCES pod(uid),
	pvc_uid VARCHAR(64) REFERENCES pvc(uid),
	container_name VARCHAR(256), -- TODO:  Change this to container_id?
	pvc_name VARCHAR(256),
	read_only bool
	);
-- Here, we assume that the container is uniquely IDed by a combination of
-- its pod and its image/parameters.  We do not containers with the same image
-- in multiple pods to be of the same container type. Essentially, we're saving
-- ourselves a join during queries and helping speed insertion--we just insert
-- one container record per container in a pod.  Otherwise, we would have to
-- check whether each container in a pod was already present in the table,
-- using a something like a combination of its image, starting parameters, and
-- ports.
CREATE TABLE IF NOT EXISTS container (
	id INT AUTO_INCREMENT PRIMARY KEY,
	pod_uid VARCHAR(64) REFERENCES pod(uid),
	name VARCHAR(256) NOT NULL,
	image VARCHAR(256) NOT NULL,
	command VARCHAR(512)
	);
CREATE TABLE IF NOT EXISTS resource_version (
	id INT AUTO_INCREMENT PRIMARY KEY,
	resource VARCHAR(64) NOT NULL,
	namespace VARCHAR(256) NOT NULL,
	resource_version VARCHAR(32) NOT NULL,
	UNIQUE KEY (resource, namespace)
	);
