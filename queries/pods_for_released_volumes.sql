SELECT inet_ntoa(n.ip_addr) as "IP Address",
	n.path as "Path",
	pv.name as "PV Name",
	p.name as "Pod Name",
	pm.container_name as "Container Name"
FROM nfs n, pv, pvc, pod_mount pm, pod p WHERE
	n.id = pv.nfs_id AND
	pv.uid = pvc.pv_uid AND
	pvc.uid = pm.pvc_uid AND
	pm.pod_uid = p.uid AND
	pvc.delete_time IS NOT NULL;
