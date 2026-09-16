{ lib, ... }:

let
  catalog = builtins.fromJSON (
    builtins.readFile ../../../catalog/devices/laptop/hp/hp-zbook-x2-g4/device.json
  );
in
{
  imports = [
    ../public-interface.nix
  ];

  oddc.hardware.catalog = {
    deviceId = lib.mkDefault catalog.id;
    facts = lib.mkDefault catalog.facts;
    blocks = lib.mkDefault catalog.blocks;
  };
}
