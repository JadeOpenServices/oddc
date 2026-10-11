# SPDX-License-Identifier: GPL-3.0-or-later

# Intel models whose firmware describes its thermal limits in DPTF tables
# need a daemon that enforces them; policy.thermal.daemon = "thermald"
# runs thermald with --adaptive, which reads those tables.
{
  config,
  lib,
  ...
}:

let
  daemon = lib.attrByPath [
    "policy"
    "thermal"
    "daemon"
  ] null config.oddc.resolved;
in
{
  config = lib.mkIf (daemon == "thermald") {
    services.thermald.enable = lib.mkDefault true;
  };
}
