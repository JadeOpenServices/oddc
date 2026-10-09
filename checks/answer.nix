# SPDX-License-Identifier: GPL-3.0-or-later

# A system built from the answer `oddc fetch` writes for one model deploys
# what a system built from the whole catalog deploys, and knows no other
# model. Only the recorded revision differs: the answer's own. An answer
# recorded at another revision than the module's fails evaluation.
{
  self,
  nixpkgs,
  pkgs,
  id,
}:

let
  inherit (nixpkgs) lib;

  oddc = lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.oddc;

  answer = pkgs.runCommand "oddc-answer" { } ''
    ${oddc} fetch --root ${self} --device ${lib.escapeShellArg id} --out $out
  '';

  evaluate =
    catalog: moduleRevision:
    lib.nixosSystem {
      inherit (pkgs.stdenv.hostPlatform) system;
      modules = [
        self.nixosModules.default
        {
          oddc.device = id;
          system.stateVersion = "26.05";
          boot.loader.grub.enable = false;
          fileSystems."/" = {
            device = "/dev/null";
            fsType = "ext4";
          };
        }
        (lib.optionalAttrs (catalog != null) { oddc.catalog = catalog; })
        { oddc.moduleRevision = lib.mkForce moduleRevision; }
      ];
    };

  etc =
    evaluated:
    pkgs.linkFarm "oddc-etc" (
      lib.mapAttrsToList (name: entry: {
        inherit name;
        path = entry.source;
      }) (lib.filterAttrs (name: _: lib.hasPrefix "oddc/" name) evaluated.config.environment.etc)
    );

  revision = lib.trim (builtins.readFile "${answer}/revision");

  failed = evaluated: map (a: a.message) (lib.filter (a: !a.assertion) evaluated.config.assertions);

  full = evaluate null null;
  fetched = evaluate answer revision;
  stale = evaluate answer "0000000000000000000000000000000000000000";
in
assert failed fetched == [ ];
assert lib.any (lib.hasPrefix "oddc.catalog was fetched at ODDC ${revision}") (failed stale);
assert fetched.config.oddc.availableModels == [ id ];
assert fetched.config.oddc.resolved == full.config.oddc.resolved;
pkgs.runCommand "oddc-answer-deployment" { } ''
  diff -r --exclude=revision ${etc full}/oddc ${etc fetched}/oddc
  [ "$(cat ${etc fetched}/oddc/revision)" = "$(cat ${answer}/revision)" ]
  touch $out
''
