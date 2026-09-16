{
  imports = [
    ../../../nixos/modules/public-interface.nix
    ./modules/battery.nix
    ./modules/boot.nix
    ./modules/fan-control.nix
  ];
}
