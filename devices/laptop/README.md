# Laptop ODDC layout

Laptop ODDC device definitions use one canonical hierarchy:

```text
laptop/common
laptop/<vendor>
laptop/<vendor>/<exact-device>
```

## Common layer

`laptop/common` contains policy and modules shared by all laptops.

It must not inherit another device layer.

## Vendor layer

A vendor layer lives directly below `laptop/`.

Examples:

```text
laptop/framework
laptop/hp
```

Vendor layers:

- inherit `laptop/common`
- contain `device.json`
- contain `default.nix`
- are not directly selectable hardware profiles

## Exact device layer

An exact supported device lives directly below its vendor.

Examples:

```text
laptop/framework/laptop-13-amd-ryzen-7040
laptop/hp/zbook-x2-g4
```

Exact-device directories:

- inherit their vendor layer
- contain `device.json`
- contain `default.nix`
- use a kebab-case hardware identity in the directory name

CPU/vendor/generation differences belong in the device name rather than
creating additional arbitrary directory levels.

## Vendor-specific auxiliary data

Vendors may contain clearly named auxiliary data directories such as:

```text
chassis-profiles/
```

Auxiliary directories are data only and must not contain `device.json`.

## Identity rule

Every device manifest ID must exactly match its path below
`oddc/devices/`.
