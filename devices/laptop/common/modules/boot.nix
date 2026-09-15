{
  config,
  pkgs,
  settings,
  ...
}:

{
  boot.loader.efi.canTouchEfiVariables = true;
  boot.loader.systemd-boot.enable = true;
  # Normal boots stay hidden; repeatedly pressing a key (F1 works with
  # systemd-boot) interrupts the hidden menu. Diagnostics deliberately leave
  # the generation chooser visible for recovery.
  boot.loader.timeout = if settings.debugFunctions then 5 else 0;
}
