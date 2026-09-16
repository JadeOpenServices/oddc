{ lib, ... }:

let
  inherit (lib) mkOption types;
in
{
  options.oddc = {
    device = mkOption {
      type = types.nullOr types.str;
      default = null;
      example = "model/framework/laptop-13-amd-ryzen-7040";
      description = ''
        Stable ODDC model entity ID selected for this machine.
      '';
    };

    overrides = mkOption {
      type = types.attrs;
      default = { };
      description = ''
        Explicit local overrides applied after resolving canonical ODDC data.
        Canonical catalog entities are never modified by these values.
      '';
    };

    availableModels = mkOption {
      type = types.listOf types.str;
      readOnly = true;
      description = "Model IDs available in the canonical ODDC entity registry.";
    };

    resolved = mkOption {
      type = types.attrs;
      readOnly = true;
      description = ''
        Fully resolved read-only ODDC view for the selected model after local
        overrides have been applied.
      '';
    };
  };
}
