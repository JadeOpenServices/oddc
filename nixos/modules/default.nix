{
  config,
  lib,
  ...
}:

let
  registry = import ../../lib/registry.nix { inherit lib; };

  cfg = config.oddc;

  selected = if cfg.device == null then null else registry.resolveEntity cfg.device;

  canonical =
    if selected == null then
      { }
    else
      lib.recursiveUpdate selected.resolved {
        model = {
          inherit (selected) id name kind;
        };
      };

  resolved = lib.recursiveUpdate canonical cfg.overrides;

  kernelParameters = builtins.attrValues (
    lib.attrByPath [
      "policy"
      "kernel"
      "parameters"
    ] { } resolved
  );

  powerProfilesEnabled = lib.attrByPath [
    "class"
    "policy"
    "power"
    "powerProfilesDaemon"
    "enable"
  ] false resolved;

  quirks = lib.attrByPath [ "quirks" ] { } resolved;

  frameworkLinux72DcnFreeze = lib.any (
    quirk: (quirk.id or null) == "quirk/framework/linux-7-2-dcn-freeze" && (quirk.enabled or false)
  ) (builtins.attrValues quirks);
in
{
  imports = [
    ./public-interface.nix
    ./capabilities/fw-fanctrl.nix
    ./quirks/framework-7040-linux-7-2-dcn-freeze.nix
    ./quirks/framework-usb-c-expansion-power.nix
    ./quirks/framework-fprintd-resume.nix
  ];

  config = lib.mkMerge [
    {
      oddc.availableModels = registry.modelIds;
      oddc.resolved = resolved;
    }

    (lib.mkIf (cfg.device != null) {
      assertions = [
        {
          assertion = lib.hasPrefix "model/" cfg.device;
          message = "oddc.device must reference a model/* entity.";
        }
      ];
    })

    (lib.mkIf (kernelParameters != [ ]) {
      boot.kernelParams = lib.mkAfter kernelParameters;
    })

    (lib.mkIf powerProfilesEnabled {
      services.power-profiles-daemon.enable = lib.mkDefault true;
    })

    (lib.mkIf frameworkLinux72DcnFreeze {
      oddc.hardware.kernel.quirks.framework7040Linux72DcnFreeze.enable = lib.mkDefault true;
    })
  ];
}
