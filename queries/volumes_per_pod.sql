select p.name as "Pod Name",
	sq.uid as "Pod UID",
	sq.total as "Volume count"
from pod p,
(
	SELECT p.uid as uid, count(pm.pvc_uid) as total FROM pod p, pod_mount pm
	WHERE p.uid = pm.pod_uid
	GROUP BY p.uid
) as sq WHERE p.uid = sq.uid;
