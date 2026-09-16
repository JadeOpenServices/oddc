{
  catalogRoot = ../catalog/entities;
  evidenceRoot = ../evidence;
  schemasRoot = ../schemas;

  mkRegistry = lib: import ./registry.nix { inherit lib; };

  nixosModules = import ../nixos/registry.nix;
}
