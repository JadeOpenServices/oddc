# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  ...
}:

# Deploy the selected model, never the whole catalog. /etc/oddc holds the
# model's reference closure in canonical layout, its evidence, the host
# overlay, and the resolved view, so `oddc` works offline.
let
  registry = import ../../lib/registry.nix {
    inherit lib;
    root = config.oddc.catalog;
  };

  cfg = config.oddc;

  entityFiles = map (id: registry.entityFiles.${id}) (registry.closure cfg.device);

  hostOverlay = {
    apiVersion = "oddc.openjade.de/v2";
    id = "host";
    kind = "host";
    targetModel = cfg.device;
    inherit (cfg) overrides;
  };
in
{
  config = lib.mkIf (cfg.deploy.enable && cfg.device != null) {
    environment.etc = lib.mkMerge [
      (lib.listToAttrs (
        map (
          file: lib.nameValuePair "oddc/catalog/entities/${file.relative}" { source = file.path; }
        ) entityFiles
      ))
      (lib.listToAttrs (
        map (file: lib.nameValuePair "oddc/evidence/${file.relative}" { source = file.path; }) (
          registry.evidenceFor cfg.device
        )
      ))
      {
        "oddc/resolved.json".text = builtins.toJSON cfg.resolved;
      }
      (lib.mkIf (cfg.revision != null) { "oddc/revision".text = "${cfg.revision}\n"; })
      (lib.mkIf (cfg.overrides != { }) {
        "oddc/host-overlay.json".text = builtins.toJSON hostOverlay;
      })
    ];
  };
}
