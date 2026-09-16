{ lib, ... }:

let
  inherit (lib) mkEnableOption mkOption types;
in
{
  options.oddc.hardware = {
    catalog = {
      deviceId = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "Stable ODDC catalog device identity.";
      };

      facts = mkOption {
        type = types.attrs;
        default = { };
        description = "Resolved portable hardware facts from the ODDC catalog.";
      };

      blocks = mkOption {
        type = types.attrs;
        default = { };
        description = "Raw device-level ODDC policy blocks.";
      };
    };

    thermal.fanControl = {
      enable = mkEnableOption "ODDC fan control";

      backend = mkOption {
        type = types.enum [ "fw-fanctrl" ];
        default = "fw-fanctrl";
      };

      defaultStrategy = mkOption {
        type = types.str;
        default = "balanced";
      };

      strategyOnDischarging = mkOption {
        type = types.str;
        default = "quiet";
      };

      strategies = mkOption {
        type = types.attrsOf types.anything;
        default = { };
        description = "Named fan strategies keyed by stable strategy ID.";
      };

      policy = {
        quietStrategy = mkOption {
          type = types.str;
          default = "quiet";
        };

        quietEnterC = mkOption {
          type = types.int;
          default = 60;
        };

        quietExitC = mkOption {
          type = types.int;
          default = 68;
        };

        performanceStrategy = mkOption {
          type = types.str;
          default = "performance";
        };

        thermalOverrideStrategy = mkOption {
          type = types.str;
          default = "max";
        };

        thermalEnterC = mkOption {
          type = types.int;
          default = 82;
        };

        thermalExitC = mkOption {
          type = types.int;
          default = 74;
        };
      };
    };
  };
}
