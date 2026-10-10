# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  ...
}:

let
  # The model has an orientation sensor it was proven with.
  hasSensor =
    lib.attrByPath [
      "hardware"
      "sensors"
      "hub"
      "provides"
      "orientation"
    ] false config.oddc.resolved == true;
in
{
  options.oddc.sensors.orientation.enable = lib.mkOption {
    type = lib.types.bool;
    default = true;
    description = ''
      Run the orientation sensor service (iio-sensor-proxy) for a model that has
      an orientation sensor. Turning the screen is left to the desktop. Has no
      effect on a model without one.
    '';
  };

  config = lib.mkIf (hasSensor && config.oddc.sensors.orientation.enable) {
    hardware.sensor.iio.enable = lib.mkDefault true;
  };
}
