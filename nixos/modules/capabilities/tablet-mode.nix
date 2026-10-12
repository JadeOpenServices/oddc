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

  # The model's policy provides tablet mode from the keyboard's presence;
  # the keyboard and switch facts alone switch nothing on.
  from = lib.attrByPath [ "policy" "input" "tabletMode" "from" ] null resolved;

  enabled = from != null && config.oddc.tabletMode.enable;

  # Only what `oddc tabletmode` reads, in resolved layout.
  tabletModeView = pkgs.writeText "oddc-tablet-mode.json" (
    builtins.toJSON {
      hardware.input.keyboard = keyboards;
      policy.input.tabletMode.from = from;
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
      keyboard, for models whose policy says so (policy.input.tabletMode.from
      = "keyboard-presence"). Has no effect elsewhere.
    '';
  };

  config = lib.mkIf enabled {
    assertions = [
      {
        assertion = from == "keyboard-presence" && signalling != { } && !switchWorks;
        message = "oddc: policy.input.tabletMode.from is ${toString from}; only keyboard-presence is known, and it needs a detachable keyboard with detachSignal bus-presence on a model without a working tablet mode switch";
      }
    ];
    boot.kernelModules = [ "uinput" ];

    # The stand's input device, by the ACPI device it belongs to.
    oddc.deviceNames.kickstand = lib.mkIf hasKickstand {
      subsystem = "input";
      rule = ''
        SUBSYSTEM=="input", KERNEL=="event*", KERNELS=="${kickstand.acpiId}:*", ENV{ID_INPUT_SWITCH}=="1", SYMLINK+="oddc/kickstand", TAG+="systemd"
      '';
    };

    systemd.services.oddc-tablet-mode = {
      description = "ODDC tablet mode switch";

      wantedBy = [ "multi-user.target" ];
      after = [ "oddc-device-names.service" ] ++ lib.optional hasKickstand "dev-oddc-kickstand.device";
      requires = [ "oddc-device-names.service" ] ++ lib.optional hasKickstand "dev-oddc-kickstand.device";

      restartTriggers = [ tabletModeView ];

      unitConfig.ConditionVirtualization = "no";

      serviceConfig = {
        ExecStart = "${config.oddc.cli.package}/bin/oddc tabletmode --resolved ${tabletModeView}" + lib.optionalString hasKickstand " --kickstand /dev/oddc/kickstand";
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
