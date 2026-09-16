{
  config,
  lib,
  pkgs,
  ...
}:

let
  fan =
    lib.attrByPath [
      "policy"
      "thermal"
      "fanControl"
    ] null config.oddc.resolved;

  enabled =
    fan != null
    && (fan.enable or true);

  normalizeStrategy =
    _: strategy:
    let
      speedMap = strategy.speedByTemperatureC or { };

      speedCurve =
        lib.sort
          (a: b: a.temp < b.temp)
          (
            lib.mapAttrsToList
              (temperature: speed: {
                temp = builtins.fromJSON temperature;
                inherit speed;
              })
              speedMap
          );
    in
    (builtins.removeAttrs strategy [ "speedByTemperatureC" ])
    // lib.optionalAttrs (speedMap != { }) {
      inherit speedCurve;
    };

  backendConfig =
    if fan == null then
      { }
    else
      {
        defaultStrategy = fan.defaultStrategy;
        strategyOnDischarging =
          fan.strategyOnDischarging or null;

        strategies =
          lib.mapAttrs
            normalizeStrategy
            fan.strategies;
      };

  runtimeConfig =
    if fan == null then
      { }
    else
      {
        schema = 1;
        enabled = true;
        backend = fan.backend;
        profile = config.oddc.device or "";
        defaultStrategy = fan.defaultStrategy;

        strategyOnDischarging =
          fan.strategyOnDischarging or null;

        strategies =
          builtins.attrNames fan.strategies;

        systemPolicy =
          fan.policy or { };
      };

  backendJson = builtins.toJSON backendConfig;
  runtimeJson = builtins.toJSON runtimeConfig;

  restartTrigger =
    pkgs.writeText
      "oddc-fan-runtime-config.json"
      runtimeJson;
in
{
  config = lib.mkIf enabled {
    assertions = [
      {
        assertion = fan.backend == "fw-fanctrl";
        message = "ODDC fan-control backend must be fw-fanctrl.";
      }
      {
        assertion =
          builtins.hasAttr
            fan.defaultStrategy
            fan.strategies;
        message =
          "ODDC default fan strategy must exist in strategies.";
      }
      {
        assertion =
          fan.strategyOnDischarging == null
          || builtins.hasAttr
            fan.strategyOnDischarging
            fan.strategies;
        message =
          "ODDC discharge fan strategy must exist in strategies.";
      }
      {
        assertion =
          (fan.policy or { }) != { };
        message =
          "ODDC fan control requires system hysteresis policy.";
      }
    ];

    environment.etc."fw-fanctrl/config.json".text =
      backendJson;

    environment.etc."gjallarOS/fan-control.json".text =
      runtimeJson;

    environment.systemPackages = [
      pkgs.fw-fanctrl
    ];

    systemd.tmpfiles.rules = [
      "d /run/gjallarOS 0755 root root - -"
    ];

    systemd.services.gjallar-fan-controller = {
      description =
        "GjallarOS ODDC fan policy controller";

      wantedBy = [
        "multi-user.target"
      ];

      restartTriggers = [
        restartTrigger
      ];

      environment.GJALLAR_FW_FANCTRL_BIN =
        "${pkgs.fw-fanctrl}/bin/fw-fanctrl";

      serviceConfig = {
        Type = "simple";

        ExecStart =
          "/run/current-system/sw/bin/gjallarctl fan controller";

        Restart = "on-failure";
        RestartSec = "2s";
      };
    };
  };
}
