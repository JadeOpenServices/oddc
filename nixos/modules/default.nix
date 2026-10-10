# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  ...
}:

let
  registry = import ../../lib/registry.nix {
    inherit lib;
    root = config.oddc.catalog;
  };

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

  moduleFiles =
    directory:
    let
      entries = builtins.readDir directory;
    in
    map (name: directory + "/${name}") (
      lib.filter (name: entries.${name} == "regular" && lib.hasSuffix ".nix" name) (
        builtins.attrNames entries
      )
    );
in
{
  imports = [
    ./public-interface.nix
    ./deployment.nix
    ./device-names.nix
  ]
  ++ moduleFiles ./capabilities
  ++ moduleFiles ./quirks;

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
        {
          assertion =
            registry.revision == null || cfg.moduleRevision == null || registry.revision == cfg.moduleRevision;
          message = ''
            oddc.catalog was fetched at ODDC ${toString registry.revision}, but the
            ODDC module is ${toString cfg.moduleRevision}. Fetch it again at the
            module's revision:
              oddc fetch --rev ${toString cfg.moduleRevision} --device ${cfg.device} --out DIR
          '';
        }
      ];
    })

    (lib.mkIf cfg.cli.enable {
      environment.systemPackages = [ cfg.cli.package ];
    })

    (lib.mkIf (kernelParameters != [ ]) {
      boot.kernelParams = lib.mkAfter kernelParameters;
    })

    (lib.mkIf powerProfilesEnabled {
      services.power-profiles-daemon.enable = lib.mkDefault true;
    })
  ];
}
