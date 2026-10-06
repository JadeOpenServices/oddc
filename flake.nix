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
      nixosModules = import ./nixos/registry.nix;

      packages = forAllSystems (pkgs: rec {
        oddcctl = pkgs.callPackage ./package.nix { };
        default = oddcctl;
      });

      checks = forAllSystems (pkgs: {
        oddcctl = self.packages.${pkgs.stdenv.hostPlatform.system}.oddcctl;
        catalog = pkgs.runCommand "oddc-catalog-valid" { } ''
          ${pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.oddcctl} validate --root ${./.}
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
              nixpkgs.lib.mapAttrsToList (name: entry: {
                inherit name;
                path = entry.source;
              }) (nixpkgs.lib.filterAttrs (name: _: nixpkgs.lib.hasPrefix "oddc/" name) evaluated.config.environment.etc)
            );
            oddcctl = pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.oddcctl;
          in
          pkgs.runCommand "oddc-deployment" { } ''
            root=${etc}/oddc
            ${oddcctl} validate --root $root
            ${oddcctl} list --root $root --kind DeviceModel > models
            [ "$(cut -f1 models | tr -d " ")" = model/framework/laptop-13-amd-ryzen-7040 ] || { cat models; exit 1; }
            ! grep -rq zbook -- ${etc}/oddc/
            ${oddcctl} resolve --root $root --device model/framework/laptop-13-amd-ryzen-7040 \
              --host $root/host-overlay.json | grep -q '"thermalEnterC": 85'
            grep -q '"thermalEnterC":85' $root/resolved.json
            touch $out
          '';
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
