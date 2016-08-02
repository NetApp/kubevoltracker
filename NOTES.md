* Invalid resource versions now being handled correctly.
* Deployments currently get handled correctly by the pod code; because a
  deployment modification creates new pods rather than editing the current ones,
  any modifications to storage in pod templates will get picked up by the add
  logic.
* Other pod modifications are a non-issue:  currently, all pod spec fields are
  immutable, except `containers[*].Image` and `spec.activeDeadlineSeconds`
* Support for PV and PVC modification is there; however, this doesn't try to
  do the right thing in the event of bound PV modification, as that's pretty
  close to undefined.
