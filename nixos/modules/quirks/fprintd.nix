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
    && (candidate ? fprintd)
  ) quirks;

  fprintd = if candidates == [ ] then null else builtins.head candidates;
  policy = if fprintd == null then { } else fprintd.fprintd;
  noTimeout = policy.noTimeout or false;
  restartAfterSleep = policy.restartAfterSleep or false;
in
{
  config = lib.mkIf (fprintd != null) (lib.mkMerge [
    {
      assertions = [
        {
          assertion = builtins.length candidates == 1;
          message = "ODDC resolved more than one enabled fprintd quirk.";
        }
      ];
    }

    (lib.mkIf noTimeout {
      systemd.services.fprintd.serviceConfig.ExecStart = lib.mkForce [
        ""
        "${pkgs.fprintd}/libexec/fprintd --no-timeout"
      ];

      systemd.services.fprintd.wantedBy = lib.mkAfter [ "multi-user.target" ];
    })

    (lib.mkIf restartAfterSleep {
      environment.etc."systemd/system-sleep/fprintd-resume-restart" = {
        mode = "0755";
        text = ''
          #!/bin/sh
          if [ "$1" = post ]; then
            ${lib.getExe' config.systemd.package "systemctl"} \
              try-restart fprintd.service >/dev/null 2>&1 || true
          fi
        '';
      };
    })
  ]);
}
