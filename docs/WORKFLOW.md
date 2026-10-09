# ODDC workflow

How ODDC reaches a machine, how people use it, and how changes get in.

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
   `host-overlay.json` when `oddc.overrides` is set, `resolved.json`,
   `inactive-quirks.json` (the model's kernel-ranged quirks it does not
   apply, because its kernel is outside their range) and the ODDC
   `revision` it was built from. Other models never reach the
   system closure.
5. **Behavior.** Generic capability and quirk modules act on resolved
   semantic values, never on vendor or model IDs.

## Channels

| Branch    | Purpose                                         | Who writes        |
|-----------|-------------------------------------------------|-------------------|
| `main`    | Stable. What consumers pull.                    | Promotion only    |
| `staging` | Integration. Proven changes only.               | Reviewed merges   |
| `verify/<vendor>/<model>` | One model's change, until it is proven. | Reviewed merges |

- Every contribution branches from the current `staging`. A pull request
  that changes a device model is retargeted to that model's
  `verify/<vendor>/<model>` branch, created from `staging` when missing;
  any other pull request is retargeted to `staging`. A change to no model,
  such as a shared quirk, may also target the verify branch of the model
  it is for.
- A maintainer or collaborator tests the model on real hardware from its
  verify branch and records the evidence there ([VERIFY.md](VERIFY.md)).
  The verify branch then merges into `staging` by pull request. CI fails
  any pull request into `staging` unless every model whose closure it
  changes is `verified`: `oddc status --since origin/staging`.
- `staging` only takes reviewed changes that pass CI.
- A maintainer promotes `staging` to `main` once its changes are tested.
  Untested or short-lived changes never reach `main`: CI fails a promotion
  unless every model is `verified` on its current closure,
  `oddc status --verified`.
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
  A change to shared components or quirks changes every model using them
  at once, and each must be proven again; a maintainer's or collaborator's
  pull request may therefore carry evidence for several models, while
  still changing at most one model's own entity.
- **New contributors wait.** Until a contributor's first pull request is
  merged, they cannot open another. A good first pull request is how
  maintainers find new collaborators.
- **Only maintainers and collaborators verify.** Anyone may add
  `documented` or `detected` evidence; `runtime-verified` and
  `hardware-validated` evidence comes only from maintainers and repository
  collaborators ([VERIFY.md](VERIFY.md)).
- A bot enforces these rules on every pull request and explains any
  rejection in a comment: `.github/workflows/pr-rules.yml`. A pull request
  with a second device or passing evidence from anyone else fails its
  check; a second open one is closed.
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

To review a revision before taking it, list what changed and of what kind:

    oddc changes --from REV [--to REV] [--json]

Each changed path is classified as `data` (catalog/), `evidence`,
`schema`, `nix`, `go`, `docs` or `other`, with its git status; a changed
or removed evidence file is flagged, since evidence is append-only. `--to`
defaults to HEAD, and `--json` gives both revisions as full commits.

For users:

    oddc detect                  # which model matches this machine
    oddc classify                # the same, as JSON with why each model did or did not match
    oddc setup                   # NixOS snippet for the matching model
    oddc fetch --out DIR         # only this machine's model, for installers
    oddc doctor                  # deployed model still matches the hardware?
    oddc status                  # is the deployed model verified by evidence?
    oddc update [--switch]       # take the newest ODDC of the stage the system follows
    oddc update --stage staging  # follow staging instead; --stage main returns to releases

`detect`, `setup` and `fetch` read the local workspace when present,
else GitHub at `main` (`--channel staging`, or `--rev COMMIT`, the full
commit ID). From GitHub they download only the model files, then only the
matched model's reference closure and evidence; nothing else of the
catalog leaves GitHub. They read it with git, not the GitHub API: the
commit's file list first, then each file when needed, each checked
against the commit. `fetch` writes that answer to `--out` in canonical layout with the
`revision` it came from: from a local checkout its commit, or `local`
when its catalog, schemas or evidence differ from that commit. `--device ID`
fetches a named model instead.
`classify` reads the local catalog (`--root`, `/etc/oddc` on a deployed
system) and this machine's DMI and PCI/USB/HID IDs from `/sys`
(`--sys DIR`), or facts from `--facts FILE`; it exits non-zero unless
exactly one model matches. `doctor` works offline on `/etc/oddc`.
`status` reports each model (`--device ID`, default the deployed model,
else all) as `verified`, `changed` or `unverified`. A model is
`verified` when its newest passing evidence (`runtime-verified` or
`hardware-validated`) was recorded on its current closure, the digest of
its resolved model. It is `changed` when passing evidence exists, but none
on that closure; new evidence must be recorded. `status` names the
evidence, the ODDC revision, BIOS and kernel it was tested with, and its
results. `--since REV` reports instead each model whose closure differs
from the one at REV, new models included, and fails unless all are
`verified`; `--verified` fails unless every model it reports is. `doctor` reports the same, and compares this machine's BIOS with
the tested one as information only; it never fails on it. `update` defaults to the flake in
`/etc/nixos` (`--flake DIR`) and rebuilds only with `--switch`. It keeps
the oddc input on the stage it follows and prints the revision before and
after. `--stage main` follows releases; `--stage staging` follows what was
merged since, such as a contributor's own model before its release. It
switches by rewriting the one `github:JadeOpenServices/oddc` URL in
`flake.nix`, so an input from elsewhere is only updated. A git flake
leaves out files git does not track, so when the flake has any, `update`
prints no rebuild command and `--switch` refuses before changing anything:
such a system has its own way to rebuild.

For contributors:

    oddc workspace               # clone or update a local catalog checkout on staging
    oddc workspace --branch verify/<vendor>/<model>   # follow a model's verify branch instead
    oddc scaffold                # draft a model entity for this machine
    oddc evidence record         # add an evidence record for this machine
    oddc contribute              # check the changes and open a pull request

The workspace is `$XDG_DATA_HOME/oddc`, by default `~/.local/share/oddc` (`--root DIR`);
`workspace --from URL` clones another source. Updating drops local files
that the followed branch now holds unchanged, such as a merged
contribution. `--branch` switches to `staging` or a verify branch, which
later updates keep following; git refuses when local changes would be
lost.

`scaffold` refuses a machine that already matches. Its draft holds the
DMI vendor, product and board name, the vendor and class entities, and
every catalog component present, placed where other models place it
(`--id ID` names it). It lists present devices it left out. It reads
`/sys` (`--sys DIR`) or `--facts FILE`, as `classify` does.

`evidence record` writes a new file below `evidence/` for the matching
model or `--device ID`; it never changes one. A match adds
`identity: pass`; `--result NAME=STATUS` adds more. `--status` defaults
to `detected`. The record names what was tested. The environment holds
only the OS name and version, the kernel version, the BIOS version and the
ODDC revision deployed in `/etc/oddc` (`--os`, `--kernel`, `--bios`,
`--deployment DIR`). When the deployed model is the recorded one,
`closure` holds its closure digest, which `status` compares, and
`inactiveQuirks` the quirks the deployment did not apply.
`runtime-verified` and `hardware-validated` are written only when the
record proves them; [VERIFY.md](VERIFY.md) says what each needs. `drivers` lists the kernel driver bound to each of the
model's components that is present, read from `/sys`. `--date` defaults
to today in UTC.

`contribute` sends only files below `catalog/` and `evidence/`, and only
when the catalog validates and evidence was only added. It builds one
commit on the newest followed branch without touching the workspace, authored by
the GitHub account's noreply address with UTC dates. It uses the
contributor's own account through `gh`, forks first without push access,
and targets the branch the workspace follows: its verify branch, else
`staging` (`--title`, `--body`). git pushes with gh's login and never asks
for a GitHub password. Without `gh`, `contribute` offers to use it from
nixpkgs for that run only, as `nix shell nixpkgs#gh` would; without a
login it offers `gh auth login`, and to sign out again at the end. Without
a terminal it asks nothing and says what to run instead.

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
records the revision it was fetched from. That must be the revision of
the oddc input: after `nix flake update oddc`, evaluation fails until the
answer is fetched again with `oddc fetch --rev` at the new input commit.
A dirty ODDC tree as input has no revision and is not compared.
`oddc.deploy.enable = false` deploys nothing under `/etc/oddc`;
`oddc.cli.enable = false` leaves out the `oddc` command.

## Maintainers

Every change is a work item in the ODDC project on plane.openjade.de and
lives on its own branch, so the history and the board tell the same
story.

- **Feature**: a top-level item with the Milestone label, holding one
  larger goal. Only one feature is open at a time.
- **Task**: a child of the feature, covering one piece of work.
- **Fix**: a child of the item it fixes, with the Fix label. A fix to
  something already merged is its own item.

Order is the sort order on the board: lowest first. Weight is never
written into titles or text.

| Item    | Branch                           | From        | Merges into |
|---------|----------------------------------|-------------|-------------|
| Feature | `feature/oddc-<n>-<slug>`        | `staging`   | `staging`   |
| Task    | `task/oddc-<n>-<slug>`           | the feature | the feature |
| Fix     | `fix/oddc-<fixed>-fixNN-<slug>`  | the feature | the feature |

1. Create the task or fix item and set it In Progress.
2. Branch from the open feature and make small commits, one logical change
   each.
3. Merge with `git merge --no-ff` into the feature, delete the branch and
   set the item Done.
4. When every child is done, merge the feature into `staging` with
   `--no-ff` and set it Done.

Promoting `staging` to `main` is `git merge --no-ff staging` on `main`,
once `oddc status --verified` passes on `staging`.

Each commit ends with a trailer naming its item, and merge messages
follow git's defaults with the merged item's trailer:

    area: what changed

    Refs: ODDC-<n>
