{
  config,
  lib,
  ...
}:

let
  quirks =
    builtins.attrValues (
      lib.attrByPath [
        "quirks"
      ] { } config.oddc.resolved
    );

  quirk =
    lib.findFirst
      (
        candidate:
        (candidate.id or null) == "quirk/framework/usb-c-expansion-power"
        && (candidate.enabled or false)
      )
      null
      quirks;

  pci = if quirk == null then { } else quirk.pci or { };
  usb = if quirk == null then { } else quirk.usb or { };
in
{
  config = lib.mkIf (quirk != null) {
    assertions = [
      {
        assertion =
          (pci.vendorId or "") != ""
          && (pci.deviceId or "") != ""
          && (pci.subsystemVendorId or "") != ""
          && (pci.subsystemDeviceId or "") != "";
        message = "Framework USB-C power quirk requires complete PCI identity.";
      }
      {
        assertion =
          (usb.vendorId or "") != ""
          && (usb.productId or "") != "";
        message = "Framework USB-C power quirk requires complete USB identity.";
      }
    ];

    services.udev.extraRules = ''
      ACTION=="add|bind", SUBSYSTEM=="pci", ATTR{vendor}=="0x${pci.vendorId}", ATTR{device}=="0x${pci.deviceId}", ATTR{subsystem_vendor}=="0x${pci.subsystemVendorId}", ATTR{subsystem_device}=="0x${pci.subsystemDeviceId}", TEST=="power/control", ATTR{power/control}="on"
      ACTION=="add|bind", SUBSYSTEM=="usb", ATTR{idVendor}=="${usb.vendorId}", ATTR{idProduct}=="${usb.productId}", TEST=="power/control", ATTR{power/control}="on"
    '';
  };
}
