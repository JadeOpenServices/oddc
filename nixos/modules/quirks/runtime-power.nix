# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  ...
}:

let
  quirks = builtins.attrValues (lib.attrByPath [ "quirks" ] { } config.oddc.resolved);

  powerQuirks = lib.filter (
    candidate:
    (candidate.enabled or false)
    && (candidate ? runtimePower)
  ) quirks;

  renderRule = quirk:
    let
      power = quirk.runtimePower;
      control = power.control or "";
      pci = power.pci or { };
      usb = power.usb or { };
      pciRule = lib.optionalString (pci != { }) ''
        ACTION=="add|bind", SUBSYSTEM=="pci", ATTR{vendor}=="0x${pci.vendorId or ""}", ATTR{device}=="0x${pci.deviceId or ""}", ATTR{subsystem_vendor}=="0x${pci.subsystemVendorId or ""}", ATTR{subsystem_device}=="0x${pci.subsystemDeviceId or ""}", TEST=="power/control", ATTR{power/control}="${control}"
      '';
      usbRule = lib.optionalString (usb != { }) ''
        ACTION=="add|bind", SUBSYSTEM=="usb", ATTR{idVendor}=="${usb.vendorId or ""}", ATTR{idProduct}=="${usb.productId or ""}", TEST=="power/control", ATTR{power/control}="${control}"
      '';
    in
    pciRule + usbRule;

  validPowerQuirk = quirk:
    let
      power = quirk.runtimePower;
      pci = power.pci or { };
      usb = power.usb or { };
      validPCI = pci == { } || (
        (pci.vendorId or "") != ""
        && (pci.deviceId or "") != ""
        && (pci.subsystemVendorId or "") != ""
        && (pci.subsystemDeviceId or "") != ""
      );
      validUSB = usb == { } || (
        (usb.vendorId or "") != ""
        && (usb.productId or "") != ""
      );
    in
    builtins.elem (power.control or "") [ "on" "auto" ]
    && (pci != { } || usb != { })
    && validPCI
    && validUSB;
in
{
  config = lib.mkIf (powerQuirks != [ ]) {
    assertions = map (quirk: {
      assertion = validPowerQuirk quirk;
      message = "ODDC runtime-power quirk requires control=on|auto and complete PCI/USB identity for each declared match.";
    }) powerQuirks;

    services.udev.extraRules = lib.concatMapStringsSep "\n" renderRule powerQuirks;
  };
}
