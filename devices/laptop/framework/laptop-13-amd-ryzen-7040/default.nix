{ lib, ... }:

{
  imports = [
    ../../../../nixos/modules/quirks/framework-7040-linux-7-2-dcn-freeze.nix
  ];

  gjallarOS.oddc.fanControl.profile = lib.mkDefault "laptop-13-amd-ryzen-7040";

  oddc.hardware.kernel.quirks.framework7040Linux72DcnFreeze.enable = lib.mkDefault true;

  boot.kernelParams = lib.mkAfter [
    "tsc=nowatchdog"
  ];
}
