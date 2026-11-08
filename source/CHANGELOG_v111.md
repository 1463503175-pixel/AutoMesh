# v1.1.1 / R8.0.4

## Continuous refinement and historical Sizing adoption

- Separate mesh-round candidate IDs from persistent live Sizing object identity.
- Permit verified prior-round Sizing to remain in the manifest even when its old element IDs no longer exist in the current remeshed candidate plan.
- Adopt reserved live `AutoMesh_QR_*` controls from ACT only after verifying exact Sizing type, Body scope, object IDs, paired coordinate system, explicit units, sphere center, radius, size and suppression state.
- Preserve every existing Sizing and coordinate system; count their current ACT-read-back budget before adding new controls.
- Add schema 3 publication rounds and per-control source plan / source mesh / batch / timestamp audit fields.
- Recompute remaining candidates against the current plan, not the lifetime control count.
- Refresh current live Sizing and coordinate-system inventory during read-only quality / mark / preview tasks and after successful publication, without a full geometry rescan.
- Show historical controls and their sphere geometry in the influence-range report; identify candidate overlap and near-duplicate risk in the review list.
- Keep near-duplicate spheres blocked; ordinary partial overlaps remain subject to user review.
- No automatic `GenerateMesh`, mesh clearing, deleting, suppressing, or modifying existing Sizing.

## Verification

- Python ACT-mock suite: 218 tests passed.
- Go tests: all passed.
- Windows x64 GUI executable: cross-build succeeded.
- ANSYS 2024 R2 live-session end-to-end validation remains pending.
