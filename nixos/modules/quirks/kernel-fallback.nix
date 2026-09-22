{
  config,
  lib,
  pkgs,
  ...
}:

let
  quirks = builtins.attrValues (lib.attrByPath [ "quirks" ] { } config.oddc.resolved);

  candidates = lib.filter (
    candidate:
    (candidate.enabled or false)
    && (candidate ? affected.kernel)
    && (candidate ? fallbackPackage)
  ) quirks;

  quirk = if candidates == [ ] then null else builtins.head candidates;
  kernel = if quirk == null then { } else quirk.affected.kernel;
  minimum = kernel.minimum or "";
  maximumBefore = kernel.maximumBefore or "";
  fallbackPackageName = if quirk == null then "" else quirk.fallbackPackage or "";

  fallbackKernelPackages =
    if fallbackPackageName != "" && builtins.hasAttr fallbackPackageName pkgs then
      builtins.getAttr fallbackPackageName pkgs
    else
      null;

  latestKernelVersion = pkgs.linuxPackages_latest.kernel.version;

  affected =
    quirk != null
    && minimum != ""
    && maximumBefore != ""
    && lib.versionAtLeast latestKernelVersion minimum
    && lib.versionOlder latestKernelVersion maximumBefore;

  safeKernelPackages =
    if !affected then
      pkgs.linuxPackages_latest
    else if fallbackKernelPackages != null then
      fallbackKernelPackages
    else
      throw "ODDC kernel fallback quirk references unavailable package ${fallbackPackageName}";
in
{
  config = lib.mkIf (quirk != null) {
    assertions = [
      {
        assertion = builtins.length candidates == 1;
        message = "ODDC resolved more than one enabled kernel fallback quirk.";
      }
      {
        assertion = minimum != "" && maximumBefore != "";
        message = "ODDC kernel fallback quirk requires affected minimum and maximumBefore versions.";
      }
      {
        assertion = fallbackKernelPackages != null;
        message = "ODDC kernel fallback quirk fallbackPackage must name an available kernel package set.";
      }
    ];

    boot.kernelPackages = lib.mkForce safeKernelPackages;
  };
}
