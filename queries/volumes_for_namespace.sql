select distinct pvc.namespace as "Namespace",
	pv.name as "PV Name",
	inet_ntoa(n.ip_addr) as "IP Address",
	n.path as "Path"
from pvc, pv, nfs n
where pvc.pv_uid=pv.uid
	and pv.nfs_id = n.id;
