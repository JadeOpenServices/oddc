# SPDX-License-Identifier: GPL-3.0-or-later

{
  config,
  lib,
  ...
}:

let
  # The model's policy runs the orientation sensor service; the sensor
  # alone switches nothing on.
  policy = lib.attrByPath [ "policy" "sensors" "orientation" ] null config.oddc.resolved;
  service = if builtins.isAttrs policy then policy.service or null else null;

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
      Run the orientation sensor service (iio-sensor-proxy) for a model whose
      policy names it (policy.sensors.orientation.service). Turning the screen
      is left to the desktop. Has no effect on other models.
    '';
  };

  config = lib.mkIf (service != null && config.oddc.sensors.orientation.enable) {
    assertions = [
      {
        assertion = service == "iio-sensor-proxy" && hasSensor;
        message = "oddc: policy.sensors.orientation names service ${toString service}, but only iio-sensor-proxy on a model whose sensor hub provides orientation is known";
      }
    ];
    hardware.sensor.iio.enable = lib.mkDefault true;
  };
}
