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
  options.oddc.sensors.orientation.present = lib.mkOption {
    type = lib.types.bool;
    readOnly = true;
    default = hasSensor;
    defaultText = lib.literalMD "whether the resolved model has an orientation sensor";
    description = ''
      Whether the resolved model has an orientation sensor it was proven with.
      For desktops that turn the screen: gate on this, not on the sensor
      service, which may run for other sensors.
    '';
  };

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
