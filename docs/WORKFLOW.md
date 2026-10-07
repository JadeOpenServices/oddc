# ODDC workflow

How ODDC reaches a machine, how people use it, and how changes get in.
Items marked *planned* are agreed design that is not implemented yet.

## Flow

1. **Catalog.** This repository holds every model, component, quirk and
   evidence record. Nobody deploys it whole.
2. **Selection.** Something picks one model for one machine: an installer
   matching DMI identity, `oddc detect`, or a person setting
   `oddc.device`. A machine without a match gets no model.
3. **Resolution.** The NixOS module resolves that model at evaluation time
   (class → vendor → family → components → model → project → host
   override) into `config.oddc.resolved`.
4. **Deployment.** The system receives only the selected model under
   `/etc/oddc`: its reference closure in canonical layout, its evidence,
   `host-overlay.json` when `oddc.overrides` is set, `resolved.json`, and
   the ODDC `revision` it was built from. Other models never reach the
   system closure.
5. **Behavior.** Generic capability and quirk modules act on resolved
   semantic values, never on vendor or model IDs.

## Channels

| Branch    | Purpose                                         | Who writes        |
|-----------|-------------------------------------------------|-------------------|
| `main`    | Stable. What consumers pull.                    | Promotion only    |
| `staging` | Integration. Every pull request targets it.     | Reviewed merges   |

- Every contribution branches from the current `staging` and targets
  `staging`. A pull request against `main` is retargeted to `staging`
  automatically (*planned*).
- `staging` only takes reviewed changes that pass CI. Changes are tested
  there, on real hardware where they touch a device.
- A maintainer promotes `staging` to `main` once its changes are tested.
  Untested or short-lived changes never reach `main`.
- Consumers follow `main`: `github:JadeOpenServices/oddc`. Testers may
  follow `github:JadeOpenServices/oddc/staging`.
- Every merge to `main` is a release; consumers pin it by commit hash.
  See [RELEASES.md](RELEASES.md).

## Contribution rules

Anyone may open a pull request. Only maintainers merge.

- **One open pull request per contributor.** Maintainers and repository
  collaborators are exempt.
- **One device per pull request.** A pull request changes at most one
  device model, with the components, quirks and evidence that model needs.
- **New contributors wait.** Until a contributor's first pull request is
  merged, they cannot open another. A good first pull request is how
  maintainers find new collaborators.
- A bot enforces these rules on every pull request and explains any
  rejection in a comment (*planned*).
- Code (Nix modules, Go) gets stricter review than catalog data: it runs as
  root on every matching machine.

The pull request text is the contributor's own, or generated from the
change: the touched entities, the validation result and evidence. A
generated text is a starting point the contributor can edit.

## The `oddc` command

One command for users and contributors. It works on any NixOS system that
imports the ODDC module, with or without anything built on top of ODDC.

On a deployed system it defaults to `/etc/oddc` and the deployed model:

    oddc list                    # entities this machine carries
    oddc resolve                 # this machine's resolved view
    oddc explain --path hardware.network.wifi.primary.driver
    oddc validate

`oddc validate --json` prints the result as JSON (`valid`, `errors`,
entity and evidence counts, `apiVersion`, `schemaVersion`, `revision`)
and, like plain `validate`, exits non-zero when the catalog is invalid.
To check any revision in CI, use the validator from that same revision:

    git -C oddc checkout REV
    nix run github:JadeOpenServices/oddc/REV -- validate --root oddc --json

Evidence is append-only. `--since REV` also fails when an evidence file
was changed or removed since the commit where HEAD branched from REV;
CI runs it on every pull request against its base branch.

For users:

    oddc detect                  # which model matches this machine
    oddc classify                # the same, as JSON with why each model did or did not match
    oddc setup                   # NixOS snippet for the matching model
    oddc fetch --out DIR         # only this machine's model, for installers
    oddc doctor                  # deployed model still matches the hardware?
    oddc update [--switch]       # update the oddc flake input, then rebuild

`detect`, `setup` and `fetch` read the local workspace when present,
else GitHub at `main` (`--channel staging`, or `--rev COMMIT`). From GitHub
they download only the model files, then only the matched model's
reference closure and evidence; nothing else of the catalog leaves
GitHub. `fetch` writes that answer to `--out` in canonical layout with the
`revision` it came from; `--device ID` fetches a named model instead.
`classify` reads the local catalog (`--root`, `/etc/oddc` on a deployed
system) and this machine's DMI and PCI/USB/HID IDs from `/sys`
(`--sys DIR`), or facts from `--facts FILE`; it exits non-zero unless
exactly one model matches. `doctor` works offline on `/etc/oddc`. `update` defaults to the flake in
`/etc/nixos` (`--flake DIR`) and rebuilds only with `--switch`.

For contributors:

    oddc workspace               # clone or update a local catalog checkout on staging
    oddc scaffold                # draft a model entity for this machine
    oddc evidence record         # add an evidence record for this machine
    oddc contribute              # check the changes and open a pull request

The workspace is `$XDG_DATA_HOME/oddc`, by default `~/.local/share/oddc` (`--root DIR`);
`workspace --from URL` clones another source. Updating drops local files
that staging now holds unchanged, such as a merged contribution.

`scaffold` refuses a machine that already matches. Its draft holds the
DMI vendor, product and board name, the vendor and class entities, and
every catalog component present, placed where other models place it
(`--id ID` names it). It lists present devices it left out. It reads
`/sys` (`--sys DIR`) or `--facts FILE`, as `classify` does.

`evidence record` writes a new file below `evidence/` for the matching
model or `--device ID`; it never changes one. A match adds
`identity: pass`; `--result NAME=STATUS` adds more. `--status` defaults
to `detected`. The environment holds only the OS name and version and
the kernel version (`--os`, `--kernel`), and `--date` defaults to today in
UTC.

`contribute` sends only files below `catalog/` and `evidence/`, and only
when the catalog validates and evidence was only added. It builds one
commit on the newest staging without touching the workspace, authored by
the GitHub account's noreply address with UTC dates. It uses the
contributor's own account through `gh`, forks first without push access,
and always targets `staging` (`--title`, `--body`).

## Using ODDC on NixOS

    # flake.nix
    inputs.oddc.url = "github:JadeOpenServices/oddc";

    # configuration.nix
    {
      imports = [ inputs.oddc.nixosModules.default ];
      oddc.device = "model/framework/laptop-13-amd-ryzen-7040";
    }

`nix flake update oddc` and a rebuild take a new catalog revision.

To keep only one machine's data, point the module at the answer
`oddc fetch` wrote; the flake input then supplies module code only:

    oddc fetch --out generated/oddc

    # configuration.nix
    oddc.catalog = ./generated/oddc;

With an answer only its model is available, and `/etc/oddc/revision`
records the revision it was fetched from.
`oddc.deploy.enable = false` deploys nothing under `/etc/oddc`;
`oddc.cli.enable = false` leaves out the `oddc` command.
