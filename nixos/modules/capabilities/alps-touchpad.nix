# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  pkgs,
  ...
}:

let
  resolved = config.oddc.resolved;

  keyboard = lib.attrByPath [
    "hardware"
    "input"
    "keyboard"
    "primary"
  ] null resolved;

  touchpad = if keyboard == null then null else keyboard.touchpad or null;

  present = touchpad != null && (touchpad.protocol or "") == "alps-u1-report-passthrough";

  enabled = present && config.oddc.touchpad.enable;

  transports = if present then builtins.attrNames (touchpad.transports or { }) else [ ];

  # "1fc9:0088" -> "1FC9:0088", as the kernel names HID devices.
  usbId = lib.toUpper (keyboard.deviceId or "");
  bluetoothId = lib.toUpper (lib.attrByPath [ "bluetooth" "deviceId" ] "" keyboard);

  # Only what `oddc touchpad` reads, in resolved layout, overrides included.
  touchpadView = pkgs.writeText "oddc-touchpad.json" (
    builtins.toJSON { hardware.input.keyboard.primary = keyboard; }
  );

  # The hidraw node of each transport. Over USB the touchpad's reports
  # come on the keyboard's fourth interface (the mouse and the vendor
  # reports); over Bluetooth there is one HID device.
  match = {
    # KERNELS and ATTRS must match one and the same parent, and the
    # interface number sits on the USB interface, not the HID device;
    # usb_id gives the hidraw node ID_USB_INTERFACE_NUM instead.
    usb = ''SUBSYSTEM=="hidraw", KERNELS=="0003:${usbId}.*", ENV{ID_USB_INTERFACE_NUM}=="03"'';
    bluetooth = ''SUBSYSTEM=="hidraw", KERNELS=="0005:${bluetoothId}.*"'';
  };
in
{
  options.oddc.touchpad.enable = lib.mkOption {
    type = lib.types.bool;
    default = true;
    description = ''
      Run the keyboard's Alps touchpad in absolute mode and offer it as a
      multitouch touchpad (two-finger scrolling and gestures over USB and
      Bluetooth). Has no effect on a model without such a touchpad.
    '';
  };

  config = lib.mkIf enabled {
    assertions = [
      {
        assertion =
          builtins.match "[0-9A-F]{4}:[0-9A-F]{4}" usbId != null
          && (!(builtins.elem "bluetooth" transports) || builtins.match "[0-9A-F]{4}:[0-9A-F]{4}" bluetoothId != null);
        message = "ODDC Alps touchpad needs the keyboard's deviceId and, for Bluetooth, bluetooth.deviceId as vvvv:pppp.";
      }
    ];

    boot.kernelModules = [ "uinput" ];

    oddc.deviceNames = lib.listToAttrs (
      map (transport: {
        name = "touchpad-${transport}";
        value = {
          subsystem = "hidraw";
          rule = ''
            ${match.${transport}}, SYMLINK+="oddc/touchpad-${transport}", TAG+="systemd", ENV{SYSTEMD_WANTS}+="oddc-touchpad@${transport}.service"
          '';
        };
      }) transports
    );

    systemd.services."oddc-touchpad@" = {
      description = "ODDC Alps touchpad (%i)";

      bindsTo = [ "dev-oddc-touchpad\\x2d%i.device" ];
      after = [
        "dev-oddc-touchpad\\x2d%i.device"
        "oddc-device-names.service"
      ];

      restartTriggers = [ touchpadView ];

      unitConfig.ConditionVirtualization = "no";

      serviceConfig = {
        ExecStart = "${config.oddc.cli.package}/bin/oddc touchpad --resolved ${touchpadView} --transport %i --device /dev/oddc/touchpad-%i";
        Restart = "on-failure";
        RestartSec = "2s";
        # On stop the service puts the touchpad back into mouse mode.
        KillSignal = "SIGTERM";
        TimeoutStopSec = "5s";

        # Root only to open root-owned device nodes; no capabilities.
        CapabilityBoundingSet = "";
        AmbientCapabilities = "";
        NoNewPrivileges = true;
        DevicePolicy = "closed";
        DeviceAllow = [
          "/dev/oddc/touchpad-usb rw"
          "/dev/oddc/touchpad-bluetooth rw"
          "/dev/uinput w"
          # To grab the input devices the generic driver made from report 7.
          "char-input r"
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
