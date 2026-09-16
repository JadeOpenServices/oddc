# ODDC model contribution template

Add a concrete hardware model under:

oddc/catalog/entities/models/<vendor>/<model>.json

Example:

oddc/catalog/entities/models/framework/laptop-13-amd-ryzen-7040.json

Use the hardware vendor's public product naming where practical so users can
find the model easily in documentation and search engines.

## Keep ownership explicit

The catalog describes what hardware is.

Validation evidence belongs under:

oddc/evidence/

Reusable hardware facts belong in component entities when they are stable,
substantial and independently useful.

Reusable exceptional behavior belongs in quirk entities.

Hardware-independent GjallarOS behavior does not belong in ODDC.

Do not create a per-device Nix module merely because a model exists.

## Required evidence

Record identity from a real machine.

At minimum verify:

- sys_vendor
- product_name
- product_version when meaningful
- board_vendor
- board_name
- board_version when meaningful
- chassis/form factor

Only declare capabilities actually observed or validated.

Do not guess firmware controls, fan controls, Secure Boot behavior, sensors,
graphics requirements, power controls or kernel quirks.

## Secure Boot

Secure Boot support is presence-based.

If a trusted Secure Boot policy exists, define the complete policy on the
authoritative entity.

If no trusted policy exists, omit it.

Do not copy another vendor or model's firmware procedure.

## Validation

Canonical validation evidence is updated only after the real-device validation
gate succeeds.

Normal probing must never rewrite canonical catalog or validation state.

## Nix behavior

Canonical models are selected through:

oddc.device = "model/<vendor>/<model>";

The public ODDC Nix module resolves the entity graph and applies reusable
capabilities and quirks.

## Auditability

Keep each entity focused.

Prefer one authoritative owner for every hardware fact or policy.

Do not duplicate generic laptop behavior inside model data.
