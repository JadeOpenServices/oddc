# ODDC NixOS integration

ODDC exposes one generic NixOS module.

Device-specific Nix modules are intentionally avoided. Canonical device
knowledge lives in the ODDC entity registry and is resolved dynamically.

Import ODDC:

    {
      imports = [
        inputs.oddc.nixosModules.default
      ];

      oddc.device = "model/framework/laptop-13-amd-ryzen-7040";
    }

The selected model is resolved through its stable entity references.

Examples:

    config.oddc.resolved.model.name

    config.oddc.resolved.vendor.name

    config.oddc.resolved.family.name

    config.oddc.resolved.hardware.processor.primary.name

    config.oddc.resolved.hardware.graphics.integrated.name

    config.oddc.resolved.hardware.network.wifi.primary.driver

    config.oddc.resolved.policy.thermal.fanControl.policy.thermalEnterC

`oddc.resolved` is generated and read-only.

Local policy changes belong under `oddc.overrides`.

Example:

    oddc.overrides.policy.thermal.fanControl.policy.thermalEnterC = 85;

The override changes the resolved host configuration without modifying the
canonical Framework model.

Available canonical models are exposed through:

    config.oddc.availableModels

Adding another ordinary device model should not require another public NixOS
module.
