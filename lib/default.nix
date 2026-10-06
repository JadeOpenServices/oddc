{
  catalogRoot = ../catalog/entities;
  evidenceRoot = ../evidence;
  schemasRoot = ../schemas;

  mkRegistry = lib: import ./registry.nix { inherit lib; };
  mkRegistryAt = lib: root: import ./registry.nix { inherit lib root; };

  nixosModules = import ../nixos/registry.nix;
}
