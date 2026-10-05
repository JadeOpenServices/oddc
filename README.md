# ODDC

ODDC is a portable, normalized hardware knowledge base.

Its central rule is:

> Every hardware fact, policy value, and implementation behavior has one
> authoritative owner.

See [CONTRIBUTING.md](CONTRIBUTING.md) before changing catalog data.

## Canonical storage

Canonical entities live under `oddc/catalog/entities/`. The registry contains
reusable classes, vendors, families, components, models, and quirks. Every
entity has stable metadata and a `data` object; references inside `data` compose
the hardware graph. Device models reference reusable entities instead of
copying their facts.

Examples:

    model/framework/laptop-13-amd-ryzen-7040
    model/hp/zbook-x2-g4

Device identity, capabilities, validated quirks, and hardware policy belong in
canonical entity data. The resolved representation is generated and is never a
second editable source of truth.

## Resolution and ownership

Resolution runs from broadest to narrowest:

1. referenced class/vendor/family/component entities
2. device model data
3. project override
4. host override

Later values override earlier values at the same stable path. Project and host
overrides change policy without modifying canonical hardware knowledge; a host
override has higher precedence than a project override. Overrideable
collections use keyed objects so their paths remain stable and explainable.

Hardware reality belongs under stable hardware and capability paths such as:

    hardware.graphics.integrated.driver
    hardware.network.wifi.primary.driver
    capabilities.chargeThresholds

Configuration policy belongs under `policy`, for example:

    policy.power.batteryProtection.shutdownPercent
    policy.power.chargeThresholds.endPercent
    policy.thermal.fanControl.policy.thermalEnterC

Validated compatibility workarounds are referenced through a model's `quirks`
object. The reusable quirk entity owns the parameters; operating-system
adapters implement only the generic semantic behavior.

Use `oddcctl explain` to inspect the final owner and override history.

## CLI

Validate the registry and evidence:

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

## NixOS

ODDC exports one generic NixOS module. Device-specific Nix modules are avoided;
quirk and capability adapters consume semantic resolved data rather than
matching vendor, model, or quirk IDs.

    {
      imports = [ inputs.oddc.nixosModules.default ];
      oddc.device = "model/framework/laptop-13-amd-ryzen-7040";
    }

The installer selects a model only when the machine's DMI identity matches one;
a machine without a match gets no model and so none of its capabilities, such as
fan control. Units that drive hardware, like fan control, also skip virtual
machines, which may inherit a host's model.

The selected model is available read-only through `config.oddc.resolved`.
Canonical model IDs are exposed through `config.oddc.availableModels`.
Machine-local policy changes belong under `oddc.overrides`, for example:

    oddc.overrides.policy.thermal.fanControl.policy.thermalEnterC = 85;

Adding another ordinary device model must not require another public NixOS
module.

## Evidence

Real-machine observations live under `oddc/evidence/` as sanitized,
append-only validation records. Evidence records what was observed and tested;
it validates canonical knowledge but does not become another hardware catalog.
Serial numbers, MAC addresses, usernames, hostnames, and other identifying
machine data must not be stored in public evidence.
