{
  catalogRoot = ../catalog;
  evidenceRoot = ../evidence;
  schemasRoot = ../schemas;

  nixosModules = import ../nixos/registry.nix;

  deviceCatalogPath =
    id:
    if id == "framework-laptop-13-amd-ryzen-7040" then
      ../catalog/devices/laptop/framework/framework-laptop-13-amd-ryzen-7040/device.json
    else if id == "hp-zbook-x2-g4" then
      ../catalog/devices/laptop/hp/hp-zbook-x2-g4/device.json
    else
      throw "Unknown ODDC device id: ${id}";
}
