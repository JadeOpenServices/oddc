# oddc

oddc is the declarative device collection consumed by GjallarOS.

It is embedded in this repository for now, but the directory is treated as an
independent source boundary so it can later move to
openDeclarativeDeviceCollectionProject without redesigning the installer.

Device policy composes from broad to specific:

1. device-class common policy
2. vendor common policy
3. concrete product/model policy
4. machine-generated hardware-configuration.nix

Machine-local device capsules and generated hardware state do not belong in
this source tree.
