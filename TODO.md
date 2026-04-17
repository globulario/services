Four services need rebuilding and redeployment:
  1. cluster_controller_server — Cases 9, 11, 12, 13, 14, 18
  2. workflow_server — Cases 10, 21
  3. node_agent_server — Cases 17, 25 + resolveBin fix
  4. backup_manager_server — Case 27

  Plus shared libraries used by multiple services:
  - event_client — Case 16 (used by all services)
  - resourcestore — Case 14 (used by controller)
  - globular_service — (pre-existing change)