# ODDC NixOS integration

The public NixOS modules are standalone consumers of the portable ODDC
catalog.

They do not import the GjallarOS runtime adapter under `oddc/devices`.

A project can use:

    inputs.oddc.url = "...";

and:

    imports = [
      inputs.oddc.nixosModules.framework-laptop-13-amd-ryzen-7040
    ];

Device defaults use ordinary Nix module priorities and may be overridden.

Example:

    oddc.hardware.thermal.fanControl.policy.thermalEnterC = 85;

The public module reads hardware defaults from the catalog and exposes them
under stable `oddc.hardware.*` option paths.

`oddc.hardware.catalog.facts` provides the machine-readable hardware facts
associated with the selected catalog device.

The portable Framework module configures the validated fw-fanctrl curves and
kernel compatibility quirk directly.

GjallarOS may layer additional operating-system policy on top through its
runtime compatibility adapter.
