{
  config,
  lib,
  pkgs,
  ...
}:

let
  fan = lib.attrByPath [
    "policy"
    "thermal"
    "fanControl"
  ] null config.oddc.resolved;

  enabled = fan != null && (fan.enable or true);

  normalizeStrategy =
    _: strategy:
    let
      speedMap = strategy.speedByTemperatureC or { };

      speedCurve = lib.sort (a: b: a.temp < b.temp) (
        lib.mapAttrsToList (temperature: speed: {
          temp = builtins.fromJSON temperature;
          inherit speed;
        }) speedMap
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
        strategyOnDischarging = fan.strategyOnDischarging;
        strategies = lib.mapAttrs normalizeStrategy fan.strategies;
      };

  configJson = builtins.toJSON backendConfig;

  restartTrigger = pkgs.writeText "oddc-fw-fanctrl-config.json" configJson;
in
{
  config = lib.mkIf enabled {
    assertions = [
      {
        assertion = fan.backend == "fw-fanctrl";
        message = "ODDC fan-control backend must be fw-fanctrl.";
      }
      {
        assertion = builtins.hasAttr fan.defaultStrategy fan.strategies;
        message = "ODDC default fan strategy must exist in strategies.";
      }
      {
        assertion = builtins.hasAttr fan.strategyOnDischarging fan.strategies;
        message = "ODDC discharge fan strategy must exist in strategies.";
      }
    ];

    environment.etc."fw-fanctrl/config.json".text = configJson;

    environment.systemPackages = [
      pkgs.fw-fanctrl
    ];

    systemd.services.oddc-fw-fanctrl = {
      description = "ODDC fw-fanctrl hardware policy";
      wantedBy = [ "multi-user.target" ];

      restartTriggers = [
        restartTrigger
      ];

      serviceConfig = {
        Type = "simple";
        Restart = "always";
        RestartSec = "2s";

        ExecStart =
          "${pkgs.fw-fanctrl}/bin/fw-fanctrl"
          + " run"
          + " --config /etc/fw-fanctrl/config.json"
          + " --silent ${fan.defaultStrategy}";
      };
    };
  };
}
