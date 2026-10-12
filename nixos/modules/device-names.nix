# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  pkgs,
  ...
}:

let
  names = config.oddc.deviceNames;

  rules = lib.concatMapStringsSep "\n" (name: name.rule) (builtins.attrValues names);

  subsystems = lib.unique (map (name: name.subsystem) (builtins.attrValues names));

  udevadm = "${config.systemd.package}/bin/udevadm";
in
{
  # Capability modules name the one device node their service needs, such as
  # /dev/oddc/quickkeys, with a udev rule. A rebuild installs a new rule but
  # does not apply it to devices that already exist, so the service would
  # wait for its node until the next boot. oddc-device-names re-applies the
  # rules to the devices of those subsystems whenever the rules change.
  options.oddc.deviceNames = lib.mkOption {
    internal = true;
    default = { };
    type = lib.types.attrsOf (
      lib.types.submodule {
        options = {
          rule = lib.mkOption {
            type = lib.types.str;
            description = "udev rule that names the device under /dev/oddc.";
          };
          subsystem = lib.mkOption {
            type = lib.types.str;
            description = "Subsystem of the device, to re-apply the rule to.";
          };
        };
      }
    );
    description = "Device nodes ODDC services need, by service.";
  };

  config = lib.mkIf (names != { }) {
    services.udev.extraRules = rules;

    systemd.services.oddc-device-names = {
      description = "Apply ODDC device names to present devices";

      wantedBy = [ "multi-user.target" ];
      after = [ "systemd-udevd.service" ];
      requires = [ "systemd-udevd.service" ];

      restartTriggers = [ (pkgs.writeText "oddc-device-names.rules" rules) ];

      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
        ExecStart = [
          "${udevadm} control --reload"
          "${udevadm} trigger --action=change ${lib.concatMapStringsSep " " (s: "--subsystem-match=${s}") subsystems}"
          "${udevadm} settle --timeout=10"
        ];
      };
    };
  };
}
