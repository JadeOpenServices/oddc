{
  config,
  lib,
  pkgs,
  settings,
  ...
}:
let
  resolvedProfile = config.gjallarOS.oddc.fanControl.profile;

  profile =
    if resolvedProfile != "" then
      builtins.fromJSON (builtins.readFile (./. + "/${resolvedProfile}/profile.json"))
    else
      throw "resolved Framework ODDC device profile has no chassis policy mapping";
  configJson = builtins.toJSON {
    "$schema" = "./config.schema.json";
    defaultStrategy = profile.defaultStrategy;
    strategyOnDischarging = profile.strategyOnDischarging;
    strategies = profile.strategies;
  };

  fanConfigRestartTrigger =
    pkgs.writeText "gjallar-framework-fan-config-trigger" configJson;
in
{
  gjallarOS.oddc.fanControl = {
    enable = true;
    backend = "fw-fanctrl";
    defaultStrategy = profile.defaultStrategy;
    strategyOnDischarging = profile.strategyOnDischarging;
    strategies = profile.strategies;

    systemPolicy = {
      quietStrategy = profile.systemPolicy.quietStrategy;
      quietEnterC = profile.systemPolicy.quietEnterC;
      quietExitC = profile.systemPolicy.quietExitC;
      performanceStrategy = profile.systemPolicy.performanceStrategy;
      thermalOverrideStrategy = profile.systemPolicy.thermalOverrideStrategy;
      thermalEnterC = profile.systemPolicy.thermalEnterC;
      thermalExitC = profile.systemPolicy.thermalExitC;
    };

  };

  environment.etc."fw-fanctrl/config.json".text = configJson;
  environment.systemPackages = [
    pkgs.fw-fanctrl
  ];

  systemd.services.gjallar-fan-controller = {
    environment.GJALLAR_FW_FANCTRL_BIN =
      "${pkgs.fw-fanctrl}/bin/fw-fanctrl";

    restartTriggers = [
      fanConfigRestartTrigger
    ];
  };

  # The Framework USB-C expansion-card controller at c1:00.3 can fail its
  # D0 -> D3hot transition, after which the USB 2 companion repeatedly logs
  # "Cannot enable" and the Realtek LAN card (0bda:8156) disappears. Keep
  # only this controller and that adapter out of runtime power saving.
  services.udev.extraRules = ''
    ACTION=="add|bind", SUBSYSTEM=="pci", ATTR{vendor}=="0x1022", ATTR{device}=="0x15b9", ATTR{subsystem_vendor}=="0xf111", ATTR{subsystem_device}=="0x0006", TEST=="power/control", ATTR{power/control}="on"
    ACTION=="add|bind", SUBSYSTEM=="usb", ATTR{idVendor}=="0bda", ATTR{idProduct}=="8156", TEST=="power/control", ATTR{power/control}="on"
  '';

  systemd.services.fprintd.serviceConfig.ExecStart = lib.mkForce [
    ""
    "${pkgs.fprintd}/libexec/fprintd --no-timeout"
  ];

  systemd.services.fprintd.wantedBy = lib.mkAfter [ "multi-user.target" ];

  environment.etc."systemd/system-sleep/restart-framework-fprintd" = {
    mode = "0755";
    text = ''
      #!/bin/sh
      if [ "$1" = post ]; then
        ${lib.getExe' config.systemd.package "systemctl"} try-restart fprintd.service >/dev/null 2>&1 || true
      fi
    '';
  };

  systemd.services.gjallar-framework-fan-curve = {
    description = "Framework fan curve control";
    wantedBy = [ "multi-user.target" ];

    restartTriggers = [
      fanConfigRestartTrigger
    ];
    serviceConfig = {
      Type = "simple";
      Restart = "always";
      RestartSec = "2s";
      ExecStart = "${pkgs.fw-fanctrl}/bin/fw-fanctrl run --config /etc/fw-fanctrl/config.json --silent ${profile.defaultStrategy}";
    };
  };
  systemd.services.gjallar-framework-battery-threshold = {
    description = "Apply Framework battery charge thresholds";
    wantedBy = [ "multi-user.target" ];
    serviceConfig.Type = "oneshot";
    script = ''
      set -eu
      for battery in /sys/class/power_supply/BAT*; do
          [ -d "$battery" ] || continue
          [ ! -w "$battery/charge_control_start_threshold" ] || printf '%s\n' ${toString profile.batteryStart} > "$battery/charge_control_start_threshold"
          [ ! -w "$battery/charge_control_end_threshold" ] || printf '%s\n' ${toString profile.batteryEnd} > "$battery/charge_control_end_threshold"
      done
    '';
  };
}
