select p.name as "Pod Name",
	pm.container_name as "Container Name",
	pvc.name as "PVC Name",
	pv.name as "PV Name",
	target_portal as "Target Portal",
	iqn as "IQN",
	lun as "LUN",
	fs_type as "FS Type"
from pod p, pod_mount pm, pvc, pv, iscsi i
where p.uid = pm.pod_uid and pm.pvc_uid = pvc.uid and pvc.pv_uid=pv.uid
	and pv.iscsi_id = i.id;
