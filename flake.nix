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
      );

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
