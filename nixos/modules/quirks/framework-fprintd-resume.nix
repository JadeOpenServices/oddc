{
  config,
  lib,
  pkgs,
  ...
}:

let
  quirks =
    builtins.attrValues (
      lib.attrByPath [
        "quirks"
      ] { } config.oddc.resolved
    );

  enabled =
    lib.any
      (
        quirk:
        (quirk.id or null) == "quirk/framework/fprintd-resume"
        && (quirk.enabled or false)
      )
      quirks;
in
{
  config = lib.mkIf enabled {
    systemd.services.fprintd.serviceConfig.ExecStart = lib.mkForce [
      ""
      "${pkgs.fprintd}/libexec/fprintd --no-timeout"
    ];

    systemd.services.fprintd.wantedBy =
      lib.mkAfter [ "multi-user.target" ];

    environment.etc."systemd/system-sleep/restart-framework-fprintd" = {
      mode = "0755";

      text = ''
        #!/bin/sh
        if [ "$1" = post ]; then
          ${lib.getExe' config.systemd.package "systemctl"} \
            try-restart fprintd.service >/dev/null 2>&1 || true
        fi
      '';
    };
  };
}
