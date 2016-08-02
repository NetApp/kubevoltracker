select p.name as "Pod Name",
	pm.container_name as "Container Name",
	pvc.name as "PVC Name",
	pv.name as "PV Name",
	inet_ntoa(n.ip_addr) as "Volume IP Address",
	n.path as "Mount Path"
from pod p, pod_mount pm, pvc, pv, nfs n
where p.uid = pm.pod_uid and pm.pvc_uid = pvc.uid and pvc.pv_uid=pv.uid
	and pv.nfs_id = n.id;
