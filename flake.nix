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
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
