# ODDC consumer contract

What a consumer can rely on without Nix: a Go program, a script, or a
service that mirrors this repository.

## Versions

- `apiVersion` (`oddc.openjade.de/v2`) versions the entity and overlay
  formats. A change an existing reader cannot read gets a new
  `apiVersion`.
- `schemaVersion` (`2.0.0`) versions evidence records and the JSON results
  of `oddc validate --json`, `oddc index` and `oddc classify`.

Both appear in every JSON result. A consumer refuses versions it does not
know instead of guessing.

## Stable IDs

An entity ID is lowercase kebab-case segments separated by `/`, such as
`model/framework/laptop-13-amd-ryzen-7040`. Validation refuses any other
form. The ID is the entity's address: it lives in
`catalog/entities/<id>.json` and nowhere else.

Once an ID is on `stable` it keeps naming the same hardware and is never
reused for something else. Renaming or removing an ID is a breaking change.

## What to read

- `oddc index --root DIR`: every entity with its kind, name, file path,
  the IDs it references and, for models, its evidence files. The index is
  generated from the catalog and never committed.
- `oddc validate --root DIR --json`: whether a revision is valid, with
  its errors. Exits non-zero when it is not. `--since REV` also fails
  when evidence was changed or removed since REV; evidence is append-only.
  Valid evidence holds nothing that identifies a machine or a person.
- `oddc changes --root DIR --from REV [--to REV] --json`: `from` and
  `to` as full commits and every changed `path` with its git `status` and
  `class`: `data`, `evidence`, `schema`, `nix`, `go`, `docs` or `other`.
  `appendOnlyViolation` marks evidence that was changed or removed.
- `oddc classify --root DIR [--sys DIR | --facts FILE]`: which model a
  machine's facts match and why. `result` is `matched` (with `model`),
  `ambiguous` (with the tied models) or `none`; it exits non-zero unless
  exactly one model matches. `candidates` lists every model at least one
  DMI field agrees with: each declared field with its expected and actual
  values, and each component with its bus, device ID and `present`
  (`null` when the facts do not cover that bus). Only DMI fields choose
  the model; components explain it. Facts are
  `{"identity": {...}, "devices": {"pci": ["vvvv:pppp"], "usb": [...], "hid": [...], "edid": ["mmm:pppp"]}}`
  with the identity fields of `MachineIdentity`. `edid` lists built-in
  panels (eDP, LVDS, DSI) by PNP manufacturer and product code from their
  EDID; the EDID serial number is never read.
- `oddc status --root DIR [--device ID] --json`: per model its `status`
  (`verified`, `changed` or `unverified`), its current `closure`
  (`sha256:` digest of the resolved model) and the `evidence` that
  decided it. Only `verified` means the model was proven on this closure.
- `oddc security --json [--resolved FILE]`: the security findings of the
  deployed model (default `/etc/oddc/resolved.json`) and the combined
  `flags`. A flag is `false` once any finding says so; a flag not in the
  map is unknown, which you must treat as unprotected. Flag names never
  change meaning. Read your security flags from here instead of keeping a
  per-model copy; see [SECURITY.md](SECURITY.md).
- The files themselves, described by `schemas/entity.schema.json`,
  `schemas/evidence.schema.json` and `schemas/overlay.schema.json`.
- In Go, package `github.com/JadeOpenServices/oddc/pkg/oddc`: `LoadRegistry`,
  `Validate`, `Registry.Index`, `Registry.ResolveModel`,
  `Registry.MatchModel`, `Registry.Classify`, `Registry.Verify`, `ReadFacts`,
  `InternalUSBDevices` and `Fetch`.

Resolved values sit under stable named paths below `hardware`,
`capabilities` and `policy`; `oddc explain` names the owner of each.

A built-in USB device carries `usbPort` on its placement: `controller`
(the path of a USB controller component in the same model), `hub`
(`usb2` or `usb3`), `port` and `connectType` (`hardwired`). IDs and
serial numbers can be cloned, so trust a USB device as internal only when
its ID appears on that port of that controller; the same ID anywhere else
is external. In Go, `InternalUSBDevices` lists them with the controller's
PCI ID, the component's `attachment` (`internal`, or `detachable` for a
part on its own connector) and `supported` (false when the entity marks it
unusable on Linux). `oddc devices --json [--root DIR] [--device ID]`
prints every part of the model: `path`, `id`, `name`, `bus`, `deviceId`,
`attachment`, `usbPort` (only for built-in USB devices), `supported`,
`proven` (`pass`, `unsupported` or `untested`, from the evidence the
model's status rests on), `protocol` and `usbIdentity`: what tells one
unit from another, `serial` `per-unit`, `placeholder` (the same on every
unit) or `none`, and an `anchor` beyond USB such as `bluetooth-address`.
Never treat a placeholder serial as identity; where nothing is per-unit,
the fixed port is the only anchor. Take built-in USB devices from
there, as the devices with a `usbPort`: a device's `bus` (`usb` or `hid`)
says how ODDC matches it, not whether it is built in. Bus numbers are not part of it: they change with boot order.

When no model matches the machine and no `--device` is given, `oddc
devices --json` answers best effort instead of failing. It reads the
machine's IDs from `/sys` (`--sys DIR`) or `--facts FILE` and prints:

- `model`: `null`.
- `devices`: every part present, as `bus`, `deviceId`, `id` (the catalog
  component with that bus and ID, `""` when there is none), `name`,
  `supported` and `unsupportedReason`, `drivers` bound now, and `proven`,
  always `untested`.
- `graphics`: every PCI display controller (class 0x03), as `slot`,
  `deviceId`, `id`, `driver` bound now, `mode` (`native` when that driver
  made a DRM card for it, `basic` when the screen runs on the firmware
  framebuffer or not at all), `removable` (behind Thunderbolt or USB4) and
  `options`. An option is a verified model that uses a GPU with the same
  PCI vendor and device ID: its `driverBranch` (that model's
  `policy.graphics.<role>.driverBranch`), the `drivers` bound to the GPU in
  its evidence, and `provenOn` (`model`, `name`, `evidence`, `kernel`,
  `os`). Only the exact chip matches, never a family. None of it is proven
  on this machine. A removable GPU has no options.
- `liveBoot.parameters`: the graphics-safety parameters (`nomodeset`,
  `DRIVER.modeset=0`) on the running kernel's command line (`--proc DIR`),
  so an installer can give the installed base what its live system needed.
- `contribute`: `guide`, `commands` (in order) and `reportEndpoint`, which
  is `null` until ODDC runs a server for reports. Send nothing while it is
  `null`, and only with the owner's consent once it is set.

Two models matching is an error, never best effort.

`oddc bootcheck --expect pci:VVVV:DDDD=DRIVER [--expect ...] [--sys DIR]`
exits non-zero unless each named PCI device has that driver bound and, for
a display controller, a DRM card of its own. An installer runs it before
`boot-complete.target` in a generation that tries a graphics option, so a
failed try falls back to the previous generation.

## Releases

Every commit on `stable` is a release. Pin one by its commit hash; `stable`
is never force-pushed, so a pinned commit stays reachable. See
[RELEASES.md](RELEASES.md).

## Not part of the contract

- Text output of commands without `--json`, and `oddc list`.
- Files outside `catalog/`, `evidence/` and `schemas/`.
- NixOS module internals beyond its documented options.
