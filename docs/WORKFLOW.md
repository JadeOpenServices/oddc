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

For users:

    oddc detect                  # which model matches this machine
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
`doctor` works offline on `/etc/oddc`. `update` defaults to the flake in
`/etc/nixos` (`--flake DIR`) and rebuilds only with `--switch`.

For contributors (*planned*):

    oddc workspace               # clone or update a local catalog checkout on staging
    oddc scaffold                # draft model and component entities from this machine
    oddc evidence record         # sanitized evidence record for this machine
    oddc contribute              # validate, branch, commit, open a pull request

`oddc contribute` uses the contributor's own GitHub account through `gh`.
Without push access it forks first. It always targets `staging`.

## Using ODDC on NixOS

    # flake.nix
    inputs.oddc.url = "github:JadeOpenServices/oddc";

    # configuration.nix
    {
      imports = [ inputs.oddc.nixosModules.default ];
      oddc.device = "model/framework/laptop-13-amd-ryzen-7040";
    }

`nix flake update oddc` and a rebuild take a new catalog revision.
`oddc.deploy.enable = false` deploys nothing under `/etc/oddc`;
`oddc.cli.enable = false` leaves out the `oddc` command.
