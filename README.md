# ODDC

ODDC is a portable, normalized hardware knowledge base.

Its central rule is:

> Every hardware fact, policy value, and implementation behavior has one
> authoritative owner.

See [CONTRIBUTING.md](CONTRIBUTING.md) before changing catalog data.

## Canonical storage

Canonical entities live under:

    oddc/catalog/entities/

The registry contains reusable vendors, families, models, components and
quirks.

Device models reference reusable entities rather than copying their facts.

Examples:

    model/framework/laptop-13-amd-ryzen-7040
    model/hp/zbook-x2-g4

## Resolution

The Go registry resolver and Nix resolver expand references into a convenient
hierarchical view.

Examples:

    vendor.name
    model.name
    hardware.processor.primary.name
    hardware.graphics.integrated.driver
    hardware.network.wifi.primary.driver
    policy.thermal.fanControl.policy.thermalEnterC

The resolved representation is generated and is not another editable source
of truth.

## CLI

Validate the canonical registry and evidence:

    oddcctl validate --root ./oddc

Resolve a model:

    oddcctl resolve \
      --root ./oddc \
      --device model/framework/laptop-13-amd-ryzen-7040

Explain a value and its ownership:

    oddcctl explain \
      --root ./oddc \
      --device model/framework/laptop-13-amd-ryzen-7040 \
      --path hardware.network.wifi.primary.driver

Project and host overrides are documented in
[docs/overrides.md](docs/overrides.md).

## NixOS

ODDC exports one generic NixOS module.

    {
      imports = [
        inputs.oddc.nixosModules.default
      ];

      oddc.device = "model/framework/laptop-13-amd-ryzen-7040";
    }

See [docs/nixos.md](docs/nixos.md).

## Evidence

Real-machine observations live under:

    oddc/evidence/

Evidence validates canonical knowledge but is not itself a second editable
hardware catalog.
