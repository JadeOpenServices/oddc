# SPDX-License-Identifier: GPL-3.0-or-later

# A deployed system carries only the selected model's closure, and the
# deployed `oddc` command reads it back. Every expectation comes from the
# catalog: the override reuses one of the model's own canonical values, and
# the fake sysfs reports the identity the model declares.
{
  self,
  nixpkgs,
  pkgs,
  registry,
  id,
}:

let
  inherit (nixpkgs) lib;

  canonical = (registry.resolveEntity id).resolved;

  # Scalar leaves of the model's hardware view, by path.
  leaves = lib.collect (leaf: leaf ? path) (
    lib.mapAttrsRecursive (path: value: { inherit path value; }) {
      inherit (canonical) hardware;
    }
  );

  # The first leaf takes the value of the next one that differs from it.
  target = builtins.head leaves;
  replacement = lib.findFirst (leaf: leaf.value != target.value) (throw "${id}: no distinct values") (
    builtins.tail leaves
  );

  evaluated = lib.nixosSystem {
    inherit (pkgs.stdenv.hostPlatform) system;
    modules = [
      self.nixosModules.default
      {
        oddc.device = id;
        oddc.overrides = lib.setAttrByPath target.path replacement.value;
        system.stateVersion = "26.05";
        boot.loader.grub.enable = false;
        fileSystems."/" = {
          device = "/dev/null";
          fsType = "ext4";
        };
      }
    ];
  };

  etc = pkgs.linkFarm "oddc-etc" (
    lib.mapAttrsToList (name: entry: {
      inherit name;
      path = entry.source;
    }) (lib.filterAttrs (name: _: lib.hasPrefix "oddc/" name) evaluated.config.environment.etc)
  );

  package = self.packages.${pkgs.stdenv.hostPlatform.system}.oddc;
  oddc = lib.getExe package;

  # The first keyed value of each DMI field, as the matcher reads it.
  dmi = lib.attrByPath [ "identity" "dmi" ] { } canonical;
  first =
    field:
    lib.optionals (dmi ? ${field}) [
      (if lib.isString dmi.${field} then dmi.${field} else lib.head (lib.attrValues dmi.${field}))
    ];
  sysfs = lib.concatLists (
    lib.mapAttrsToList (file: field: map (value: { inherit file value; }) (first field)) {
      sys_vendor = "systemVendor";
      product_name = "productName";
      product_version = "productVersion";
      board_vendor = "boardVendor";
      board_name = "boardName";
      board_version = "boardVersion";
    }
  );
  battery = lib.attrByPath [ "class" "formFactor" ] null canonical == "laptop";

  # Enabled kernel-ranged quirks whose range misses the newest kernel are
  # not applied.
  latest = pkgs.linuxPackages_latest.kernel.version;
  inactive = lib.sort lib.lessThan (
    lib.attrNames (
      lib.filterAttrs (
        _: quirk:
        (quirk.enabled or false)
        && quirk ? affected.kernel
        && !(
          lib.versionAtLeast latest quirk.affected.kernel.minimum
          && lib.versionOlder latest quirk.affected.kernel.maximumBefore
        )
      ) (canonical.quirks or { })
    )
  );

  others = lib.remove id registry.modelIds;
  check = lib.escapeShellArgs [
    "--argjson"
    "path"
    (builtins.toJSON target.path)
    "--argjson"
    "want"
    (builtins.toJSON replacement.value)
  ];
in
pkgs.runCommand "oddc-deployment" { nativeBuildInputs = [ pkgs.jq ]; } ''
  root=${etc}/oddc
  ${oddc} validate --root $root
  ${oddc} list --root $root --kind DeviceModel > models
  [ "$(cut -f1 models | tr -d " ")" = ${id} ] || { cat models; exit 1; }
  ${lib.concatMapStrings (other: ''
    ! grep -rqF -- ${lib.escapeShellArg other} $root/
  '') others}

  # Without --device and --host, resolve uses the deployed model and overlay.
  ${oddc} resolve --root $root \
    | jq -e ${check} '.resolved | getpath($path) == $want'
  jq -e ${check} 'getpath($path) == $want' $root/resolved.json

  jq -e --argjson want ${lib.escapeShellArg (builtins.toJSON inactive)} '. == $want' \
    $root/inactive-quirks.json

  mkdir -p sys/class/dmi/id
  ${oddc} doctor --root $root --sys sys && exit 1
  ${lib.concatMapStrings (entry: ''
    echo ${lib.escapeShellArg entry.value} > sys/class/dmi/id/${entry.file}
  '') sysfs}
  ${lib.optionalString battery ''
    mkdir -p sys/class/power_supply/BAT0
    echo Battery > sys/class/power_supply/BAT0/type
  ''}
  ${oddc} doctor --root $root --sys sys

  # The model's kernel, after its quirks, exists in this nixpkgs.
  echo ${evaluated.config.boot.kernelPackages.kernel.version} > /dev/null

  [ "$(cat $root/revision)" = ${self.rev or self.dirtyRev or "unknown"} ]
  ${lib.boolToString (
    lib.elem package.name (map (p: p.name or "") evaluated.config.environment.systemPackages)
  )}
  touch $out
''
