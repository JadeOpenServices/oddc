# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  pkgs,
  ...
}:

let
  resolved = config.oddc.resolved;

  keyboards = lib.attrByPath [ "hardware" "input" "keyboard" ] { } resolved;

  # Keyboards whose leaving the bus is the model's tablet mode signal.
  signalling = lib.filterAttrs (
    _: keyboard:
    builtins.isAttrs keyboard
    && (keyboard.attachment or null) == "detachable"
    && (keyboard.detachSignal or null) == "bus-presence"
  ) keyboards;

  switchWorks = lib.attrByPath [ "capabilities" "tabletModeSwitch" ] false resolved == true;

  # The firmware switch that reports the stand, if the model has one.
  kickstand = lib.attrByPath [ "capabilities" "kickstandSwitch" ] null resolved;
  hasKickstand = builtins.isAttrs kickstand && (kickstand.acpiId or "") != "";

  enabled = signalling != { } && !switchWorks && config.oddc.tabletMode.enable;

  # Only what `oddc tabletmode` reads, in resolved layout.
  tabletModeView = pkgs.writeText "oddc-tablet-mode.json" (
    builtins.toJSON {
      hardware.input.keyboard = keyboards;
      capabilities = {
        tabletModeSwitch = switchWorks;
      }
      // lib.optionalAttrs hasKickstand { kickstandSwitch = kickstand; };
    }
  );
in
{
  options.oddc.tabletMode.enable = lib.mkOption {
    type = lib.types.bool;
    default = true;
    description = ''
      Provide a tablet mode switch from the presence of the model's detachable
      keyboard, for models without a working one. Has no effect elsewhere.
    '';
  };

  config = lib.mkIf enabled {
    boot.kernelModules = [ "uinput" ];

    # The stand's input device, by the ACPI device it belongs to.
    services.udev.extraRules = lib.mkIf hasKickstand ''
      SUBSYSTEM=="input", KERNEL=="event*", KERNELS=="${kickstand.acpiId}:*", SYMLINK+="oddc/kickstand", TAG+="systemd"
    '';

    systemd.services.oddc-tablet-mode = {
      description = "ODDC tablet mode switch";

      wantedBy = [ "multi-user.target" ];
      after = [ "systemd-udevd.service" ] ++ lib.optional hasKickstand "dev-oddc-kickstand.device";
      requires = lib.optional hasKickstand "dev-oddc-kickstand.device";

      restartTriggers = [ tabletModeView ];

      unitConfig.ConditionVirtualization = "no";

      serviceConfig = {
        ExecStart = "${config.oddc.cli.package}/bin/oddc tabletmode --resolved ${tabletModeView}";
        Restart = "on-failure";
        RestartSec = "2s";

        # Root only to open /dev/uinput and the stand's switch; no capabilities.
        CapabilityBoundingSet = "";
        AmbientCapabilities = "";
        NoNewPrivileges = true;
        DevicePolicy = "closed";
        DeviceAllow = [ "/dev/uinput w" ] ++ lib.optional hasKickstand "/dev/oddc/kickstand r";
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        # No PrivateNetwork: the kernel sends uevents only to the host
        # network namespace. Netlink is the only socket family it may open.
        RestrictAddressFamilies = [ "AF_NETLINK" ];
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
