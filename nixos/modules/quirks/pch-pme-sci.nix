# SPDX-License-Identifier: GPL-3.0-or-later

# Intel PCH root ports can turn a PME from the device behind them into an
# ACPI GPE (PMEX, config 0xD8 bit 31; status PMSX, 0xDC bit 31). When the
# firmware's handler for that GPE never clears PMSX, the level-triggered GPE
# fires until reboot. A pchPmeSci quirk with action "disable" names such a
# port by its full PCI identity; this module clears PMSX and then PMEX on
# exactly those ports, at boot and after every resume. Linux still sees the
# device's wakeups through its own PME polling.
{
  config,
  lib,
  pkgs,
  ...
}:

let
  quirks = builtins.attrValues (lib.attrByPath [ "quirks" ] { } config.oddc.resolved);

  sciQuirks = lib.filter (
    candidate: (candidate.enabled or false) && (candidate ? pchPmeSci)
  ) quirks;

  hex4 = value: builtins.match "[0-9a-f]{4}" value != null;

  validQuirk =
    quirk:
    let
      sci = quirk.pchPmeSci;
      port = sci.rootPort or { };
    in
    (sci.action or "") == "disable"
    && builtins.all (field: hex4 (port.${field} or "")) [
      "vendorId"
      "deviceId"
      "subsystemVendorId"
      "subsystemDeviceId"
    ];

  # "8086 9d12 103c 824c" per port, matched against sysfs.
  identities = map (
    quirk:
    let
      port = quirk.pchPmeSci.rootPort;
    in
    "${port.vendorId} ${port.deviceId} ${port.subsystemVendorId} ${port.subsystemDeviceId}"
  ) sciQuirks;

  apply = pkgs.writeShellScript "oddc-pch-pme-sci" ''
    set -eu
    wanted=${lib.escapeShellArg (lib.concatStringsSep "\n" identities)}
    found=0
    for dev in /sys/bus/pci/devices/*; do
      id="$(
        for f in vendor device subsystem_vendor subsystem_device; do
          v="$(cat "$dev/$f")"
          printf '%s ' "''${v#0x}"
        done
      )"
      id="''${id% }"
      if printf '%s\n' "$wanted" | ${pkgs.gnugrep}/bin/grep -qxF "$id"; then
        port="$(basename "$dev")"
        # PMEX off first so no new PME is routed, then clear a latched PMSX.
        ${pkgs.pciutils}/bin/setpci -s "$port" d8.L=00000000:80000000
        ${pkgs.pciutils}/bin/setpci -s "$port" dc.L=80000000
        echo "$port: PME to GPE routing off (MPC $(${pkgs.pciutils}/bin/setpci -s "$port" d8.L))"
        found=$((found + 1))
      fi
    done
    if [ "$found" -eq 0 ]; then
      echo "no root port matches the ODDC pchPmeSci quirk" >&2
      exit 1
    fi
  '';
in
{
  config = lib.mkIf (sciQuirks != [ ]) {
    assertions = map (quirk: {
      assertion = validQuirk quirk;
      message = "ODDC pchPmeSci quirk requires action=disable and a full rootPort identity (vendorId, deviceId, subsystemVendorId, subsystemDeviceId as 4 lowercase hex digits).";
    }) sciQuirks;

    systemd.services.oddc-pch-pme-sci = {
      description = "ODDC: stop the PCH routing PMEs to an unhandled GPE";
      wantedBy = [
        "sysinit.target"
        "post-resume.target"
      ];
      after = [ "post-resume.target" ];
      before = [ "sysinit.target" ];
      unitConfig = {
        DefaultDependencies = false;
        ConditionVirtualization = "no";
      };
      serviceConfig = {
        Type = "oneshot";
        ExecStart = apply;
        CapabilityBoundingSet = [ "CAP_SYS_ADMIN" ];
        NoNewPrivileges = true;
        PrivateNetwork = true;
        ProtectHome = true;
        ProtectSystem = "strict";
        RestrictAddressFamilies = "none";
        SystemCallArchitectures = "native";
      };
    };
  };
}
