Short-term Issues
==================

* Make sure that this correctly ingests events that happened offline; it looks
  like watches may not provide added events when started anew (NOTE THAT IT
  LOOKS LIKE THIS HAS BEEN FIXED IN 1.2).
* Verify the behavior where namespace watches appear to do prefix matching.
* Test in a container.
* Containerize the build process and automate deployment (probably via a pod
  definition).
* Test coverage isn't great; consider adding more mocks.  Not sure how
  necessary this is for the DB code, though.
* Consider clearing DB resources between tests in `watcher_mysql_test.go`.
* Add tests for ISCSI PVs and pods that use the mock dbmanager.
* Refactor `watcher_mysql_test` to make the set-up/teardown functionality
  common.
* Implement more robust recovery when the most recent RV has been discarded
  by the API server.  Currently, any resources that were deleted during the
  down period will remain open; these should be closed and given a timestamp.

Moderate Issues
================

* Robustness testing:  How does this hold up under scale?
* Performance testing:  Does SQL and the implementation hold up?
* Rewrite the listener code to use the Kubernetes APIs directly?

Long-term Issues
================

* Alternate back-ends?  Is a graph DB more appropriate?  Should we support a
  variety?
* Front-end:  figure out what would work best for this.  Integration with
  Kubedash/Heapster?
