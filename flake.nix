# SPDX-License-Identifier: GPL-3.0-or-later

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
      registry = import ./lib/registry.nix { inherit (nixpkgs) lib; };
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      lib = import ./lib;
      # Through the flake, a deployment from the flake's own catalog records
      # the ODDC revision it was built from; an answer records its own and
      # must come from the same revision as the module.
      nixosModules = builtins.mapAttrs (_: module: { config, ... }: {
        imports = [ module ];
        oddc.moduleRevision = nixpkgs.lib.mkDefault (self.rev or null);
        oddc.revision = nixpkgs.lib.mkIf (!builtins.pathExists (config.oddc.catalog + "/revision")) (
          nixpkgs.lib.mkDefault (self.rev or self.dirtyRev or null)
        );
      }) (import ./nixos/registry.nix);

      packages = forAllSystems (pkgs: rec {
        oddc = pkgs.callPackage ./package.nix { };
        default = oddc;
      });

      checks = forAllSystems (
        pkgs:
        {
          oddc = self.packages.${pkgs.stdenv.hostPlatform.system}.oddc;
          catalog = pkgs.runCommand "oddc-catalog-valid" { } ''
            ${pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.oddc} validate --root ${./.}
            touch $out
          '';
          # Every catalog model deploys only its own closure, and the deployed
          # command reads it back. Expectations come from the catalog itself.
        }
        // nixpkgs.lib.listToAttrs (
          map (id: {
            name = "deployment-" + builtins.replaceStrings [ "/" ] [ "-" ] id;
            value = import ./checks/deployment.nix {
              inherit
                self
                nixpkgs
                pkgs
                registry
                id
                ;
            };
          }) registry.modelIds
        )
        // nixpkgs.lib.listToAttrs (
          map (id: {
            name = "answer-" + builtins.replaceStrings [ "/" ] [ "-" ] id;
            value = import ./checks/answer.nix {
              inherit
                self
                nixpkgs
                pkgs
                id
                ;
            };
          }) registry.modelIds
        )
      );

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
