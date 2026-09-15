{
  default = import ./modules/public-interface.nix;

  framework-laptop-13-amd-ryzen-7040 = import ./modules/devices/framework-laptop-13-amd-ryzen-7040.nix;

  hp-zbook-x2-g4 = import ./modules/devices/hp-zbook-x2-g4.nix;
}
