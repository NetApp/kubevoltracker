select pv.name as "PV Name",
	pvc.name as "PVC Name",
	sq.total as "Pod count"
from pv, pvc,
(
	SELECT pm.pvc_uid as uid, count(pm.pod_uid) as total FROM pod_mount pm
	GROUP BY pm.pvc_uid
) as sq WHERE pv.uid = pvc.pv_uid and pvc.uid = sq.uid;
