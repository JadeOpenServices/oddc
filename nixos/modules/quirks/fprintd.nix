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
  recoverUsbAfterSleep = policy.recoverUsbAfterSleep or false;

  fingerprintBus = lib.attrByPath [
    "hardware"
    "security"
    "fingerprint"
    "primary"
    "bus"
  ] "" config.oddc.resolved;

  fingerprintDeviceId = lib.attrByPath [
    "hardware"
    "security"
    "fingerprint"
    "primary"
    "deviceId"
  ] "" config.oddc.resolved;

  fingerprintIdParts = lib.splitString ":" fingerprintDeviceId;
  fingerprintVendor =
    if builtins.length fingerprintIdParts == 2
    then builtins.elemAt fingerprintIdParts 0
    else "";
  fingerprintProduct =
    if builtins.length fingerprintIdParts == 2
    then builtins.elemAt fingerprintIdParts 1
    else "";
in
{
  config = lib.mkIf (fprintd != null) (lib.mkMerge [
    {
      assertions = [
        {
          assertion = builtins.length candidates == 1;
          message = "ODDC resolved more than one enabled fprintd quirk.";
        }
        {
          assertion =
            !recoverUsbAfterSleep
            || (
              fingerprintBus == "usb"
              && fingerprintVendor != ""
              && fingerprintProduct != ""
            );
          message =
            "ODDC fprintd USB resume recovery requires a resolved USB fingerprint deviceId.";
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
      environment.etc."systemd/system-sleep/fprintd-resume-recovery" = {
        mode = "0755";
        text = ''
          #!/bin/sh

          cache=/run/fprintd-resume-xhci-controller
          vendor=${lib.escapeShellArg fingerprintVendor}
          product=${lib.escapeShellArg fingerprintProduct}

          find_device() {
            for vendor_file in /sys/bus/usb/devices/*/idVendor; do
              [ -f "$vendor_file" ] || continue
              dev="''${vendor_file%/idVendor}"

              [ "$(${pkgs.coreutils}/bin/cat "$dev/idVendor" 2>/dev/null)" = "$vendor" ] ||
                continue
              [ "$(${pkgs.coreutils}/bin/cat "$dev/idProduct" 2>/dev/null)" = "$product" ] ||
                continue

              printf '%s\n' "$dev"
              return 0
            done

            return 1
          }

          find_controller() {
            node="$(${pkgs.coreutils}/bin/readlink -f "$1")"

            while [ -n "$node" ] && [ "$node" != / ]; do
              base="''${node##*/}"

              case "$base" in
                ????\:??\:??.?)
                  if [ -L "$node/driver" ] &&
                     [ "$(${pkgs.coreutils}/bin/basename \
                       "$(${pkgs.coreutils}/bin/readlink -f "$node/driver")")" = xhci_hcd ]; then
                    printf '%s\n' "$base"
                    return 0
                  fi
                  ;;
              esac

              node="''${node%/*}"
            done

            return 1
          }

          cache_controller() {
            dev="$(find_device 2>/dev/null || true)"
            [ -n "$dev" ] || return 0

            controller="$(find_controller "$dev" 2>/dev/null || true)"
            [ -n "$controller" ] || return 0

            printf '%s\n' "$controller" > "$cache"
          }

          wait_until_authorized() {
            attempt=0

            while [ "$attempt" -lt 15 ]; do
              dev="$(find_device 2>/dev/null || true)"

              if [ -n "$dev" ] &&
                 [ -r "$dev/authorized" ] &&
                 [ "$(${pkgs.coreutils}/bin/cat "$dev/authorized" 2>/dev/null)" = 1 ]; then
                return 0
              fi

              attempt=$((attempt + 1))
              ${pkgs.coreutils}/bin/sleep 1
            done

            return 1
          }

          recover_controller() {
            [ -r "$cache" ] || return 1

            controller="$(${pkgs.coreutils}/bin/cat "$cache" 2>/dev/null)"

            case "$controller" in
              ????\:??\:??.?)
                ;;
              *)
                return 1
                ;;
            esac

            pci="/sys/bus/pci/devices/$controller"

            [ -L "$pci/driver" ] || return 1
            [ "$(${pkgs.coreutils}/bin/basename \
              "$(${pkgs.coreutils}/bin/readlink -f "$pci/driver")")" = xhci_hcd ] ||
              return 1

            ${lib.getExe' config.systemd.package "systemctl"} \
              stop fprintd.service >/dev/null 2>&1 || true

            printf '%s\n' "$controller" \
              > /sys/bus/pci/drivers/xhci_hcd/unbind

            ${pkgs.coreutils}/bin/sleep 2

            printf '%s\n' "$controller" \
              > /sys/bus/pci/drivers/xhci_hcd/bind

            return 0
          }

          if [ "$1" = pre ]; then
            ${lib.optionalString recoverUsbAfterSleep ''
              cache_controller
            ''}
            exit 0
          fi

          [ "$1" = post ] || exit 0

          ${lib.optionalString recoverUsbAfterSleep ''
            if ! find_device >/dev/null 2>&1; then
              recover_controller || true
            fi

            wait_until_authorized || true
          ''}

          ${lib.getExe' config.systemd.package "systemctl"} \
            restart fprintd.service >/dev/null 2>&1 || true
        '';
      };
    })
  ]);
}
