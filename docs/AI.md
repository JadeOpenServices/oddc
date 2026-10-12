# Instructions for AI assistants

AI use is encouraged for ODDC contributions. Point your assistant (Claude
Code, Codex or your own) at this file; `CLAUDE.md` and `AGENTS.md` in the
repository root load it for the tools that read them. These are the rules
the maintainers' own assistant works by. A contribution whose assistant
drifted from them (guessed data, fallbacks, fake test data, workarounds
sold as fixes) will most likely be rejected and has to be redone.

The rules in [CONTRIBUTING.md](../CONTRIBUTING.md) and the docs in this
folder are part of these instructions. Read them before changing anything.

## What ODDC is

A hardware knowledge base. It records what hardware a machine has, what
works, what needs a quirk, and the evidence that proves it. Software-side
policy (apps, services, credentials, AI endpoints) belongs to the projects
that consume ODDC, not here. Decline it and say where it belongs.

## The ideology

- **Exact model or nothing.** A machine matches one model or none. No
  family or vendor fallback, no generic support for an unknown machine.
  Consumers fail closed.
- **One owner per fact.** Every fact, policy value and behaviour has one
  entity that owns it. Models compose by reference; nothing inherits and
  nothing overrides inside the catalog.
- **Support is proven, not assumed.** A model is supported only with passing
  evidence from the real machine on its current closure. When the closure
  changes, it needs new evidence.
- **Behaviour is never triggered by facts.** Facts are information. Anything
  a consumer does (switch a GPU, enrol keys, map buttons) is an explicit
  per-model policy or quirk reference, added after it was tested on that
  model and proven by evidence. "Do X when fact Y is present" is a guess.
- **Shared definitions, separate proof.** Several models may reference one
  quirk or security finding; each still proves it with its own evidence.
- **Security findings only lower trust.** See [SECURITY.md](SECURITY.md).
  Only measured weaknesses of hardware or firmware become findings; an
  unknown (a check that could not run) is not one, and system settings
  belong to the operating system.
- **No lists in the catalog.** Collections are keyed objects, each entry
  under a name of its own, so every value has a stable path.
- **Decide strictly, ask on gaps.** Decide open questions by these rules
  without offering choices the rules already answer. When a case does not
  fit, ask the maintainer before dropping it, so the rules can be widened
  on purpose.

## How to work

- **Research first.** Before designing a schema, rule or behaviour, check
  how established projects handle it (nixos-hardware, fwupd/LVFS,
  linux-hardware.org, systemd hwdb, kernel and NixOS docs, OSV) and cite
  the sources. Skip it for wording and for bugs with a known cause.
- **Root cause, not workarounds.** Find out why something breaks (logs,
  real files, git history, a reproduction) before fixing it. No retries,
  self-healing guards or "re-apply on every boot" as the fix. If the cause
  is out of reach, say so and label a workaround as one.
- **No fake data.** Tests use the real catalog and real tools (real git,
  local repositories). Never invent machines, DMI values or service
  doubles.
- **A device that does not work on Linux:** read the vendor's driver package
  (INF hardware IDs, drivers, settings) as a source; never run its
  executables. Name the package in sources (vendor, package ID, version).
- **Devices with only a vendor driver for another OS:** capture their
  traffic from a VM on the machine itself: an official evaluation image,
  no activation tools, exactly one USB device passed through, no network
  while it is attached, the vendor driver from the vendor's own package,
  and a stop before anything that may change device state (ownership,
  flash writes). Captures with serials or biometric data stay local.
- **Read-only first on firmware.** Reading firmware, flash or registers
  comes first. Every write (flash, ownership, settings) is the machine
  owner's explicit decision, with a recovery path ready.
- **Use the established sources first.** fwupd (`fwupdmgr get-devices`,
  `fwupdmgr security` for host security attributes), systemd's hwdb,
  sysfs, the kernel's own reports. Their data is a source of facts for
  discovery and evidence, never a switch for behaviour or a claim of
  support: only a model's own evidence proves that.
- **Identifiers stay local.** Probes may read serials, addresses and
  keys to learn whether a part has a per-unit identity; they record only
  that (present, length, placeholder or not), never the value.
- **Explain security warnings fully:** what it is, what an attacker could do,
  how bad it is, the fix, and when it gets fixed. Never call one harmless
  without saying why.

## The toolchain

Commands follow the ideology too (CONTRIBUTING, "The toolchain follows the
same rules"): one inventory (`oddc devices`) that every tool filters, no
vendor, model or device ID in code, behaviour only where the model
references it, protocol tools instead of device tools, every behaviour
proven by evidence, fail closed, debug tools labelled and read-only.

## Testing on real hardware

- Test that each part **works**, not only that it is present: a ping, a
  sound heard by the microphone, frames from a camera, a write to the disk.
- Parts that are built in but not proven on Linux (`proven` other than
  `pass` in `oddc devices`) are their own class: recorded and identified,
  not used, until evidence proves them.
- An automatic round ends with one verdict. A skipped check is not a
  pass; "all tests pass" means nothing failed and nothing was skipped.
- When a test cannot tell (another interface on the same network, a
  device not plugged in), it says so and what to change, instead of a bare
  failure.
- Hand tests are guided: one step at a time, the next step only on Enter or
  a tap, every event logged under the step that caused it, devices that
  disappear (a detached keyboard) picked up again, and never any typing
  logged. The log goes to a file you read yourself.
- Commands the person has to run (root, a physical action): give the exact
  command, one at a time, without `#` comments.
- Ask before anything hard to undo on the person's machine: reboots,
  firmware or flash writes, device ownership, deleting files you did not
  create.

## Privacy

Nothing that identifies a person or one machine goes into the repository:
no names, user names, e-mail addresses, home paths, host names, serial
numbers, MAC or IP addresses, network names, filesystem UUIDs, keys or
licence codes (firmware product keys included). `oddc validate` rejects
the common ones in evidence; check catalog sources, docs, commit messages
and test data yourself. Keep raw logs and captures on the machine and
share scrubbed copies only.

## Git and review

- Branches: `feature/oddc-<n>-<slug>` from `staging`, `task/oddc-<n>-<slug>`
  from the feature, `fix/oddc-<n>-fixNN-<slug>` for fixes. Merge with
  `git merge --no-ff` and a `Refs: ODDC-<n>` trailer. Commit messages are
  "area: summary".
- `staging` and `main` change only through pull requests with passing CI.
  Never push to them directly and never bypass branch protection.
- Evidence is append-only: never edit or delete a record.
- Before you commit: `gofmt -l .`, `go vet ./...`, `go test ./...`,
  `oddc validate`. A new test must fail on the old code.
- Change the docs in the same change. Keep them short: a contributor should
  find what they need in a few minutes of reading.
- When a change affects what consumers must do (commands, outputs,
  `/etc/oddc` contents, verification rules), tell the consumer projects.

## Writing

Write like a person: say what a thing is and does, concretely. No
slogans, no filler, no narrating to the reader. This applies to docs,
catalog sources, commits and pull requests.
