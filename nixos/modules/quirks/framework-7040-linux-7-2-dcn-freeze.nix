{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.oddc.hardware.kernel.quirks.framework7040Linux72DcnFreeze;

  latestKernelVersion = pkgs.linuxPackages_latest.kernel.version;

  safeKernelPackages =
    if lib.versionAtLeast latestKernelVersion "7.2" && lib.versionOlder latestKernelVersion "7.3" then
      pkgs.linuxPackages_7_1
    else
      pkgs.linuxPackages_latest;
in
{
  options.oddc.hardware.kernel.quirks.framework7040Linux72DcnFreeze = {
    enable = lib.mkEnableOption "Framework Laptop 13 AMD 7040 Linux 7.2 DCN freeze workaround";
  };

  config = lib.mkIf cfg.enable {
    boot.kernelPackages = lib.mkForce safeKernelPackages;
  };
}
