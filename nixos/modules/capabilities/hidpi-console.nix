# SPDX-License-Identifier: GPL-3.0-or-later

# A built-in panel above 200 ppi makes the 8x16 console font, the LUKS
# prompt and the boot menu unreadably small. From the model's panel this
# module picks the 16x32 Terminus console font (early, so the initrd's
# password prompt uses it too) and, when the model names one, the boot
# menu's console mode (policy.boot.consoleMode). Both are defaults that a
# system configuration can override.
{
  config,
  lib,
  pkgs,
  ...
}:

let
  resolved = config.oddc.resolved;

  panel = lib.attrByPath [
    "hardware"
    "display"
    "internal"
    "primary"
  ] null resolved;

  width = lib.attrByPath [ "resolution" "width" ] 0 panel;
  widthMm = lib.attrByPath [ "size" "widthMm" ] 0 panel;

  # Pixels per inch along the width, in whole numbers.
  ppi = if panel == null || widthMm == 0 then 0 else (width * 254) / (widthMm * 10);

  dense = ppi >= 200;

  consoleMode = lib.attrByPath [
    "policy"
    "boot"
    "consoleMode"
  ] null resolved;
in
{
  config = lib.mkMerge [
    (lib.mkIf dense {
      console = {
        font = lib.mkDefault "ter-v32n";
        packages = [ pkgs.terminus_font ];
        earlySetup = lib.mkDefault true;
      };
    })
    (lib.mkIf (consoleMode != null) {
      boot.loader.systemd-boot.consoleMode = lib.mkDefault (toString consoleMode);
    })
  ];
}
