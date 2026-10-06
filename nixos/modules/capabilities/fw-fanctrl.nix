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

  backendRestartTrigger =
    pkgs.writeText
      "oddc-fan-backend-config.json"
      backendJson;

  waitForBackend =
    pkgs.writeShellScript "fan-control-backend-ready" ''
      for attempt in $(${pkgs.coreutils}/bin/seq 1 50); do
        if ${pkgs.fw-fanctrl}/bin/fw-fanctrl \
          --output-format JSON \
          print current \
          >/dev/null 2>&1
        then
          exit 0
        fi

        ${pkgs.coreutils}/bin/sleep 0.1
      done

      echo "fw-fanctrl control socket did not become ready" >&2
      exit 1
    '';
  controller = config.oddc.fanControl.controller;
in
{
  # ODDC owns the hardware policy and the fw-fanctrl backend. The policy
  # controller that applies thermalEnterC/hysteresis is the consumer's own
  # program; it reads the runtime policy from /etc/oddc/fan-control.json.
  options.oddc.fanControl.controller = {
    command = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = "Command that applies the ODDC fan policy; null runs only the backend.";
    };
    environment = lib.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = { };
      description = "Environment for the policy controller.";
    };
  };

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

    environment.etc."oddc/fan-control.json".text =
      runtimeJson;

    environment.systemPackages = [
      pkgs.fw-fanctrl
    ];

    systemd.services.fan-control-backend = {
      description =
        "fw-fanctrl backend";

      wantedBy = [
        "multi-user.target"
      ];

      restartTriggers = [
        backendRestartTrigger
      ];

      # A VM built from this model has no embedded controller to drive.
      unitConfig.ConditionVirtualization = "no";

      script = ''
        exec ${pkgs.fw-fanctrl}/bin/fw-fanctrl \
          run \
          --config /etc/fw-fanctrl/config.json \
          --silent ${lib.escapeShellArg fan.defaultStrategy}
      '';

      serviceConfig = {
        Type = "simple";
        ExecStartPost = waitForBackend;

        Restart = "on-failure";
        RestartSec = "2s";
      };
    };

    systemd.services.fan-policy-controller = lib.mkIf (controller.command != null) {
      description =
        "ODDC fan policy controller";

      requires = [
        "fan-control-backend.service"
      ];

      after = [
        "fan-control-backend.service"
      ];

      partOf = [
        "fan-control-backend.service"
      ];

      wantedBy = [
        "multi-user.target"
      ];

      restartTriggers = [
        restartTrigger
      ];

      unitConfig.ConditionVirtualization = "no";

      environment = {
        ODDC_FAN_CONFIG = "/etc/oddc/fan-control.json";
        ODDC_FW_FANCTRL_BIN = "${pkgs.fw-fanctrl}/bin/fw-fanctrl";
      }
      // controller.environment;

      serviceConfig = {
        Type = "simple";

        ExecStart = controller.command;

        Restart = "on-failure";
        RestartSec = "2s";
      };
    };
  };
}
