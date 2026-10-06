{
  config,
  lib,
  pkgs,
  ...
}:

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

    deploy.enable = mkOption {
      type = types.bool;
      default = true;
      description = ''
        Install the selected model under /etc/oddc: its reference closure in
        canonical catalog layout, its evidence, the host overlay, and
        resolved.json. Other models never reach the system closure.
      '';
    };

    revision = mkOption {
      type = types.nullOr types.str;
      default = null;
      example = "e94f6bc";
      description = ''
        ODDC revision this system was built from, recorded in
        /etc/oddc/revision. The flake module sets it from the flake input.
      '';
    };

    cli.enable = mkOption {
      type = types.bool;
      default = config.oddc.deploy.enable;
      defaultText = lib.literalExpression "config.oddc.deploy.enable";
      description = ''
        Install the `oddc` command. On a deployed system it defaults to
        /etc/oddc and the deployed model.
      '';
    };

    cli.package = mkOption {
      type = types.package;
      default = pkgs.callPackage ../../package.nix { };
      defaultText = lib.literalExpression "oddc";
      description = "Package providing the `oddc` command.";
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
