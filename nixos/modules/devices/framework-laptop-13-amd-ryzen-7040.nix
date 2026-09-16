{
  lib,
  ...
}:

let
  catalog = builtins.fromJSON (
    builtins.readFile ../../../catalog/devices/laptop/framework/framework-laptop-13-amd-ryzen-7040/device.json
  );

  fan = catalog.blocks.thermal."fan-control";

  policy = fan.policy;
in
{
  imports = [
    ../public-interface.nix
    ../capabilities/fw-fanctrl.nix
    ../quirks/framework-7040-linux-7-2-dcn-freeze.nix
  ];

  oddc.hardware.catalog = {
    deviceId = lib.mkDefault catalog.id;
    facts = lib.mkDefault catalog.facts;
    blocks = lib.mkDefault catalog.blocks;
  };

  oddc.hardware.thermal.fanControl = {
    enable = lib.mkDefault true;

    backend = lib.mkDefault fan.backend;

    defaultStrategy = lib.mkDefault fan.defaultStrategy;

    strategyOnDischarging = lib.mkDefault fan.strategyOnDischarging;

    strategies = lib.mkDefault fan.strategies;

    policy = {
      quietStrategy = lib.mkDefault policy.quietStrategy;

      quietEnterC = lib.mkDefault policy.quietEnterC;

      quietExitC = lib.mkDefault policy.quietExitC;

      performanceStrategy = lib.mkDefault policy.performanceStrategy;

      thermalOverrideStrategy = lib.mkDefault policy.thermalOverrideStrategy;

      thermalEnterC = lib.mkDefault policy.thermalEnterC;

      thermalExitC = lib.mkDefault policy.thermalExitC;
    };
  };

  oddc.hardware.kernel.quirks.framework7040Linux72DcnFreeze.enable = lib.mkDefault true;

  boot.kernelParams = lib.mkAfter [
    "tsc=nowatchdog"
  ];
}
