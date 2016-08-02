SELECT pvc.name as "PVC Name",
	pv.name as "PV Name",
	IF(pv.nfs_id !=0, 'NFS', IF(pv.iscsi_id != 0, 'ISCSI', 'Unknown'))
		as 'Volume Type'
FROM pvc, pv WHERE pvc.pv_uid = pv.uid AND pvc.uid NOT IN (
	SELECT pm.pvc_uid FROM pod_mount pm WHERE pm.pvc_uid IS NOT NULL
);
