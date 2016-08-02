select p.name as "Pod Name",
	pm.container_name as "Container Name",
	pvc.name as "PVC Name",
	pv.name as "PV Name",
	IF(pv.nfs_id !=0, 'NFS', IF(pv.iscsi_id != 0, 'ISCSI', 'Unknown'))
		as 'Volume Type'
from pod p, pod_mount pm, pvc, pv
where p.uid = pm.pod_uid and pm.pvc_uid = pvc.uid and pvc.pv_uid=pv.uid;
