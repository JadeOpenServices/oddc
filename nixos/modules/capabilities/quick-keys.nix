# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  pkgs,
  ...
}:

let
  resolved = config.oddc.resolved;

  quickKeys = lib.attrByPath [
    "hardware"
    "input"
    "quickKeys"
    "primary"
  ] null resolved;

  mappable = quickKeys != null && (lib.attrByPath [ "access" "mappable" ] false quickKeys);

  enabled = mappable && config.oddc.quickKeys.enable;

  # "03f0:0a56" -> "03F0:0A56", as the kernel names HID devices.
  hidId = lib.toUpper (quickKeys.deviceId or "");

  # Only what `oddc quickkeys` reads, in resolved layout, overrides included.
  quickKeysView = pkgs.writeText "oddc-quick-keys.json" (
    builtins.toJSON {
      hardware.input.quickKeys.primary = quickKeys;
      policy.input.quickKeys = lib.attrByPath [ "policy" "input" "quickKeys" ] { } resolved;
    }
  );
in
{
  options.oddc.quickKeys.enable = lib.mkOption {
    type = lib.types.bool;
    default = true;
    description = ''
      Turn the model's Quick Keys into key presses (policy.input.quickKeys.keymap)
      on a virtual keyboard. Has no effect on a model without mappable Quick Keys.
    '';
  };

  config = lib.mkIf enabled {
    assertions = [
      {
        assertion = builtins.match "[0-9A-F]{4}:[0-9A-F]{4}" hidId != null;
        message = "ODDC Quick Keys need a deviceId of the form vvvv:pppp.";
      }
    ];

    boot.kernelModules = [ "uinput" ];

    # The one hidraw node of this device gets a stable name and starts the
    # service; no other HID device is touched.
    oddc.deviceNames.quickKeys = {
      subsystem = "hidraw";
      rule = ''
        SUBSYSTEM=="hidraw", KERNELS=="*:${hidId}.*", SYMLINK+="oddc/quickkeys", TAG+="systemd", ENV{SYSTEMD_WANTS}+="oddc-quick-keys.service"
      '';
    };

    systemd.services.oddc-quick-keys = {
      description = "ODDC Quick Keys";

      bindsTo = [ "dev-oddc-quickkeys.device" ];
      after = [
        "dev-oddc-quickkeys.device"
        "oddc-device-names.service"
      ];

      restartTriggers = [ quickKeysView ];

      unitConfig.ConditionVirtualization = "no";

      serviceConfig = {
        ExecStart = "${config.oddc.cli.package}/bin/oddc quickkeys --resolved ${quickKeysView} --device /dev/oddc/quickkeys --state /run/oddc/quick-keys";
        # The active preset, for desktops to show: /run/oddc/quick-keys/preset.
        RuntimeDirectory = "oddc/quick-keys";
        RuntimeDirectoryMode = "0755";
        Restart = "on-failure";
        RestartSec = "2s";

        # Root only to open the two root-owned device nodes; no capabilities.
        CapabilityBoundingSet = "";
        AmbientCapabilities = "";
        NoNewPrivileges = true;
        DevicePolicy = "closed";
        DeviceAllow = [
          "/dev/oddc/quickkeys r"
          "/dev/uinput w"
        ];
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        PrivateNetwork = true;
        RestrictAddressFamilies = "none";
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectKernelLogs = true;
        ProtectControlGroups = true;
        ProtectClock = true;
        ProtectHostname = true;
        RestrictNamespaces = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";
        SystemCallFilter = [
          "@system-service"
          "~@privileged"
        ];
      };
    };
  };
}
