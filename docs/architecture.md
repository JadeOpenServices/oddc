# ODDC architecture

ODDC separates portable hardware knowledge from operating-system implementation.

## Portable catalog

`catalog/` contains machine-readable hardware knowledge.

Catalog identity is permanent and independent from directory layout.

## GjallarOS runtime adapter

GjallarOS consumes the resolved ODDC catalog directly.

Device-specific identity, capabilities, validated quirks, and hardware policy
belong in ODDC. Generic operating-system behavior belongs in GjallarOS modules.
No machine-profile compatibility layer sits between those two boundaries.

## Layer order

Configuration resolves from broadest to narrowest:

1. class
2. vendor
3. family
4. device
5. project override
6. host override

Family layers are optional.

Later layers override earlier layers.

## Facts

`facts` describe hardware reality.

Facts may contain arrays when the hardware genuinely has multiple values.

## Blocks

`blocks` contain named, overrideable configuration and compatibility policy.

Overrideable collections use keyed objects instead of positional arrays.

Good:

    blocks.thermal.fan-control.strategies.quiet

Bad:

    blocks.thermal.fan-control.strategies[0]

Consumers must be able to address configuration by a stable path without
guessing line numbers or array positions.

## Evidence

`evidence/` contains sanitized append-only real-machine validation records.

Evidence records describe what was actually observed and tested on real
hardware at a specific point in time.

Machine-identifying information such as serial numbers, MAC addresses,
usernames and hostnames must not be stored in public evidence.
