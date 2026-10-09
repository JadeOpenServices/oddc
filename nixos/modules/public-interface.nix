# SPDX-License-Identifier: GPL-3.0-or-later

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

    catalog = mkOption {
      type = types.path;
      default = ../..;
      defaultText = lib.literalExpression "the ODDC source tree";
      example = lib.literalExpression "./generated/oddc";
      description = ''
        Catalog the model is read from: the whole ODDC source tree, or an
        answer `oddc fetch` wrote for this machine (its model's reference
        closure, evidence and revision). With an answer, only that model
        is available and its recorded revision is deployed.
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
      default =
        let
          file = config.oddc.catalog + "/revision";
        in
        if builtins.pathExists file then lib.trim (builtins.readFile file) else null;
      defaultText = lib.literalExpression "the catalog's recorded revision, if any";
      example = "e94f6bc";
      description = ''
        ODDC revision this system was built from, recorded in
        /etc/oddc/revision. An answer records its own; otherwise the flake
        module sets it from the flake input.
      '';
    };

    moduleRevision = mkOption {
      type = types.nullOr types.str;
      default = null;
      internal = true;
      description = ''
        Commit of the ODDC source this module comes from, set by the flake
        module from its input; null for a dirty tree. An answer recorded at
        any other revision fails evaluation.
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

    inactiveQuirks = mkOption {
      type = types.listOf types.str;
      default = [ ];
      internal = true;
      description = ''
        Keys of the resolved model's enabled kernel-ranged quirks that this
        system does not apply, because its kernel is outside their range.
      '';
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
