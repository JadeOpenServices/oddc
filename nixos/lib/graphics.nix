# SPDX-License-Identifier: GPL-3.0-or-later

{ lib }:
resolved:
let
  graphics = lib.attrByPath [ "hardware" "graphics" ] { } resolved;
  discrete = graphics.discrete or { };
  integrated = graphics.integrated or { };
  discreteDriver = discrete.driver or "";
  integratedDriver = integrated.driver or "";
  selected = if discreteDriver != "" then discrete else integrated;
  vendorName = lib.toLower (lib.attrByPath [ "vendor" "name" ] "" selected);
  vendor =
    if vendorName == "nvidia" || discreteDriver == "nvidia" then
      "nvidia"
    else if vendorName == "amd" || integratedDriver == "amdgpu" then
      "amd"
    else if vendorName == "intel" || integratedDriver == "i915" then
      "intel"
    else
      "";
in
{
  inherit
    discrete
    discreteDriver
    integrated
    integratedDriver
    vendor
    ;
  type =
    if discreteDriver != "" && integratedDriver != "" then
      "hybrid"
    else if discreteDriver != "" then
      "discrete"
    else if integratedDriver != "" then
      "integrated"
    else
      "none";
  compute = lib.attrByPath [ "capabilities" "compute" ] false selected;
}
