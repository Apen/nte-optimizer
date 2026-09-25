# Sanitized optimizer fixtures

`sanitized_inventory.json` is a fabricated inventory used by optimizer tests.
Its 24 modules use the geometry and area distribution observed in aggregate
from a read-only local inventory: 4 two-cell, 12 three-cell, and 8 four-cell
modules. Each synthetic module has two main stats and four substats, matching
the observed record shape. The fixture covers all twelve production geometries.

All item identifiers, set identifiers, character identifiers, equipment
placements, cartridges, and stat values in this file are synthetic. No source
inventory rows, account identifiers, item identifiers, or imported JSON were
copied into the repository. The fixture is representative test data, not a
snapshot of an account.
