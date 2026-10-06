# ODDC consumer contract

What a consumer can rely on without Nix: a Go program, a script, or a
service that mirrors this repository.

## Versions

- `apiVersion` (`oddc.openjade.de/v2`) versions the entity and overlay
  formats. A change an existing reader cannot read gets a new
  `apiVersion`.
- `schemaVersion` (`2.0.0`) versions evidence records and the JSON results
  of `oddc validate --json` and `oddc index`.

Both appear in every JSON result. A consumer refuses versions it does not
know instead of guessing.

## Stable IDs

An entity ID is lowercase kebab-case segments separated by `/`, such as
`model/framework/laptop-13-amd-ryzen-7040`. Validation refuses any other
form. The ID is the entity's address: it lives in
`catalog/entities/<id>.json` and nowhere else.

Once an ID is on `main` it keeps naming the same hardware and is never
reused for something else. Renaming or removing an ID is a breaking change.

## What to read

- `oddc index --root DIR`: every entity with its kind, name, file path,
  the IDs it references and, for models, its evidence files. The index is
  generated from the catalog and never committed.
- `oddc validate --root DIR --json`: whether a revision is valid, with
  its errors. Exits non-zero when it is not.
- The files themselves, described by `schemas/entity.schema.json`,
  `schemas/evidence.schema.json` and `schemas/overlay.schema.json`.
- In Go, package `github.com/JadeOpenServices/oddc`: `LoadRegistry`,
  `Validate`, `Registry.Index`, `Registry.ResolveModel`,
  `Registry.MatchModel` and `Fetch`.

Resolved values sit under stable named paths below `hardware`,
`capabilities` and `policy`; `oddc explain` names the owner of each.

## Not part of the contract

- Text output of commands without `--json`, and `oddc list`.
- Files outside `catalog/`, `evidence/` and `schemas/`.
- NixOS module internals beyond its documented options.
