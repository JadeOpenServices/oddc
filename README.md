# ODDC

ODDC is a portable hardware knowledge catalog with reusable NixOS integration.

## Public structure

    catalog/    portable hardware facts and named policy blocks
    evidence/   sanitized append-only real-machine validation
    schemas/    machine-readable public contracts
    nixos/      reusable NixOS modules
    lib/        directly importable Nix helpers
    devices/    GjallarOS runtime compatibility adapter
    docs/       architecture and override documentation

## Portable device identity

Catalog IDs are stable public identities and do not depend on directory layout.

Examples:

    framework-laptop-13-amd-ryzen-7040
    hp-zbook-x2-g4

## NixOS use

A consuming project can import:

    oddc/nixos/modules/devices/framework-laptop-13-amd-ryzen-7040.nix

The public override namespace is:

    oddc.hardware.*

Example:

    oddc.hardware.thermal.fanControl.policy.thermalEnterC = 85;

The GjallarOS runtime namespace remains available internally for compatibility.

## Override order

    class
    vendor
    family
    device
    project
    host

Use `oddcctl explain` to inspect the effective value and its provenance.
