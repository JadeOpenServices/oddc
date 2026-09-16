# oddc device contribution template

Copy this template into the appropriate path below:

oddc/devices/<class>/<vendor>/<model>/

Example:

oddc/devices/laptop/hp/zbook-x2-g4/

The directory path and manifest id must match exactly.

For example:

directory:
oddc/devices/laptop/hp/zbook-x2-g4/

device.json:
"id": "laptop/hp/zbook-x2-g4"

## Keep the profile small

Only add behavior unique to this device.

Before adding a module, check whether it belongs in:

1. <class>/common
2. <class>/<vendor>
3. the concrete device

Do not duplicate generic laptop or vendor behavior in a model directory.

## Required evidence

Record hardware identity from the real machine.

At minimum verify:

- sys_vendor
- product_name
- product_version when meaningful
- board_vendor
- board_name
- board_version when meaningful
- chassis/form factor

Only declare hardware capabilities actually observed or validated.

Do not guess firmware controls, fan controls, Secure Boot behavior, sensors,
graphics requirements, power controls or kernel quirks.

## Secure Boot

Secure Boot behavior is device-specific.

Document firmware prerequisites and limitations when known.

If Secure Boot behavior has not been validated, leave it explicitly
unvalidated. Do not copy another vendor's firmware procedure.

## Validation

lastValidated metadata is not manually advanced merely because a newer release
exists.

It is updated only after the real-device validation gate succeeds.

Normal users must not update upstream validation state.

## Files

A typical concrete device may contain:

device.json
README.md
default.nix
graphics.nix
thermal.nix
tablet.nix
secure-boot.nix
quirks.nix

Only create files the device actually needs.

## Auditability

Keep each module focused.

Avoid giant device files containing unrelated policy.

Prefer declarative Nix over shell where possible.

Any imperative behavior should have a clear hardware reason and a focused
validation test.
