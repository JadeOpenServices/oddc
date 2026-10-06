{
  description = "ODDC: a portable, normalized hardware knowledge base for NixOS";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-26.05";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      lib = import ./lib;
      # Through the flake, a deployment records the ODDC revision it was
      # built from.
      nixosModules = builtins.mapAttrs (_: module: {
        imports = [ module ];
        oddc.revision = nixpkgs.lib.mkDefault (self.rev or self.dirtyRev or null);
      }) (import ./nixos/registry.nix);

      packages = forAllSystems (pkgs: rec {
        oddc = pkgs.callPackage ./package.nix { };
        default = oddc;
      });

      checks = forAllSystems (pkgs: {
        oddc = self.packages.${pkgs.stdenv.hostPlatform.system}.oddc;
        catalog = pkgs.runCommand "oddc-catalog-valid" { } ''
          ${pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.oddc} validate --root ${./.}
          touch $out
        '';
        # A deployed system carries only the selected model's closure.
        deployment =
          let
            evaluated = nixpkgs.lib.nixosSystem {
              inherit (pkgs.stdenv.hostPlatform) system;
              modules = [
                self.nixosModules.default
                {
                  oddc.device = "model/framework/laptop-13-amd-ryzen-7040";
                  oddc.overrides.policy.thermal.fanControl.policy.thermalEnterC = 85;
                  system.stateVersion = "26.05";
                  boot.loader.grub.enable = false;
                  fileSystems."/" = {
                    device = "/dev/null";
                    fsType = "ext4";
                  };
                }
              ];
            };
            etc = pkgs.linkFarm "oddc-etc" (
              nixpkgs.lib.mapAttrsToList
                (name: entry: {
                  inherit name;
                  path = entry.source;
                })
                (
                  nixpkgs.lib.filterAttrs (
                    name: _: nixpkgs.lib.hasPrefix "oddc/" name
                  ) evaluated.config.environment.etc
                )
            );
            oddc = pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.oddc;
          in
          pkgs.runCommand "oddc-deployment" { } ''
            root=${etc}/oddc
            ${oddc} validate --root $root
            ${oddc} list --root $root --kind DeviceModel > models
            [ "$(cut -f1 models | tr -d " ")" = model/framework/laptop-13-amd-ryzen-7040 ] || { cat models; exit 1; }
            ! grep -rq zbook -- ${etc}/oddc/
            # Without --device and --host, resolve uses the deployed model and overlay.
            ${oddc} resolve --root $root | grep -q '"thermalEnterC": 85'
            grep -q '"thermalEnterC":85' $root/resolved.json
            mkdir -p sys/class/dmi/id
            echo Framework > sys/class/dmi/id/sys_vendor
            echo "Laptop 13 (AMD Ryzen 7040Series)" > sys/class/dmi/id/product_name
            ${oddc} doctor --root $root --sys sys
            echo Other > sys/class/dmi/id/sys_vendor
            ! ${oddc} doctor --root $root --sys sys
            [ "$(cat $root/revision)" = ${self.rev or self.dirtyRev or "unknown"} ]
            ${
              if
                nixpkgs.lib.elem self.packages.${pkgs.stdenv.hostPlatform.system}.oddc.name (
                  map (p: p.name or "") evaluated.config.environment.systemPackages
                )
              then
                "true"
              else
                "false"
            }
            touch $out
          '';
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
