{ config, lib, pkgs, ... }:

let
  cfg = config.gjallarOS.oddc.fanControl;

  runtime = builtins.toJSON {
    schema = 1;
    enabled = cfg.enable;
    backend = cfg.backend;
    profile = cfg.profile;
    defaultStrategy = cfg.defaultStrategy;
    strategyOnDischarging = cfg.strategyOnDischarging;
    strategies = builtins.attrNames cfg.strategies;
    systemPolicy = cfg.systemPolicy;
  };

  runtimeRestartTrigger =
    pkgs.writeText "gjallar-fan-runtime-config-trigger" runtime;
in
{
  options.gjallarOS.oddc.fanControl = {
    enable = lib.mkEnableOption "ODDC-managed fan control";

    backend = lib.mkOption {
      type = lib.types.str;
      default = "";
    };

    profile = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Backend-specific fan profile selected by the resolved ODDC device leaf.";
    };

    defaultStrategy = lib.mkOption {
      type = lib.types.str;
      default = "";
    };

    strategyOnDischarging = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
    };

    strategies = lib.mkOption {
      type = lib.types.attrs;
      default = { };
    };

    systemPolicy = {
      quietStrategy = lib.mkOption {
        type = lib.types.str;
        default = "";
      };

      quietEnterC = lib.mkOption {
        type = lib.types.int;
        default = 60;
      };

      quietExitC = lib.mkOption {
        type = lib.types.int;
        default = 68;
      };

      performanceStrategy = lib.mkOption {
        type = lib.types.str;
        default = "";
      };

      thermalOverrideStrategy = lib.mkOption {
        type = lib.types.str;
        default = "";
      };

      thermalEnterC = lib.mkOption {
        type = lib.types.int;
        default = 85;
      };

      thermalExitC = lib.mkOption {
        type = lib.types.int;
        default = 75;
      };
    };
  };

  config = lib.mkIf cfg.enable {
    environment.etc."gjallarOS/fan-control.json".text = runtime;

    systemd.tmpfiles.rules = [
      "d /run/gjallarOS 0755 root root - -"
    ];

    systemd.services.gjallar-fan-controller = {
      description = "GjallarOS ODDC fan policy controller";
      restartTriggers = [ runtimeRestartTrigger ];
      wantedBy = [ "multi-user.target" ];
      after = [ "multi-user.target" ];
      serviceConfig = {
        Type = "simple";
        ExecStart = "/run/current-system/sw/bin/gjallarctl fan controller";
        Restart = "on-failure";
        RestartSec = "2s";
      };
    };
  };
}
