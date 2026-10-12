# Contributing to ODDC

ODDC is a portable hardware knowledge base.

It is not a collection of duplicated Nix profiles.

The central design rule is:

> Every hardware fact, policy value, and implementation behavior has one authoritative owner.

Everything else must reference, resolve, generate, or observe that authoritative value.

This document defines how ODDC data is structured, how contributors add hardware, and which forms of duplication are forbidden.

---

## Using AI

AI use is encouraged. Point your assistant (Claude Code, Codex or your own)
at [docs/AI.md](docs/AI.md): it holds the rules the maintainers' own
assistant works by, and `CLAUDE.md` and `AGENTS.md` load it automatically
for the tools that read them. A contribution whose AI drifted from these
rules and principles will most likely be rejected and has to be redone.

## 1. Mental model

ODDC separates four different kinds of information:

1. **Catalog**
   - What a piece of hardware is.
   - Canonical hardware identity and relationships.
   - Declarative recommended policy.

2. **Evidence**
   - What was actually observed on real machines.
   - Append-only validation history.
   - Evidence never becomes canonical merely because it was observed once.

3. **Implementation**
   - How an operating system applies catalog policy.
   - NixOS modules, quirks, drivers, services, and other executable behavior.

4. **Overrides**
   - Intentional project or host-local changes.
   - Overrides never rewrite canonical catalog data.

Do not mix these responsibilities.

---

## 2. Single source of truth

"Single source of truth" does not mean one giant JSON document.

It means that any individual fact has exactly one authoritative owner.

Examples:

| Knowledge | Authoritative owner |
| --- | --- |
| Framework vendor metadata | vendor entity |
| Framework Laptop 13 family facts | family entity |
| AMD Ryzen 7 7840U specifications | processor entity |
| Radeon 780M identifiers | graphics entity |
| RTL8852BE identifiers and normal Linux driver | Wi-Fi component entity |
| Framework 7040 machine composition | device model entity |
| Framework Secure Boot policy | Framework vendor or model policy, whichever scope is actually correct |
| Linux 7.2 Framework DCN workaround metadata | quirk entity |
| Nix implementation of that workaround | one Nix quirk module |
| Observed real-machine result | evidence record |

A contributor must never copy one of those values into a second editable source merely because another consumer needs it.

Consumers resolve references instead.

### Ownership rules

- **Exact model or nothing.** A machine matches exactly one device model by
  its identity, or it matches none. There is no closest model, no family
  fallback and no generic profile; a consumer without a match fails closed.
- **One owner per fact.** Every resolved value comes from exactly one
  entity file. `oddc explain` names it, and the tests fail any value whose
  named owner does not hold it.
- **No inheritance, no override.** An entity holds a fact or references the
  entity that does. A value next to a `ref` adds to what the referenced
  entity holds; it never redefines a value that entity holds. Nothing
  fills in a value an entity does not have.
- **A shared entity owns a fact only when every model using it is evidenced
  for it.** A vendor, family, class or component entity is used by every
  model that references it, so a fact placed there is a claim about all of
  them. Put it there only when each of those models is proven with it;
  otherwise it belongs to the one model it was proven on. Changing a shared
  entity changes the closure of every model using it, and each must be
  proven again ([docs/WORKFLOW.md](docs/WORKFLOW.md)).

---

## 3. Composition over inheritance

ODDC prefers explicit composition and references over deep inheritance.

Avoid chains such as:

    class
      -> vendor
        -> family
          -> model
            -> runtime profile

when those layers exist only to merge arbitrary JSON into one object.

Instead, a device model explicitly identifies what it contains:

    device model
      -> vendor reference
      -> family reference
      -> processor reference
      -> graphics reference
      -> Wi-Fi reference
      -> quirk references
      -> model-specific policy

This makes ownership obvious.

There is no inheritance and no default. A family, vendor or class entity
appears in the resolved view under its own reference (`family`, `vendor`,
`class`), never merged into the model's own values.

---

## 4. Stable entity identity

Every canonical entity has a stable ID.

Examples:

    vendor/framework
    family/framework/laptop-13
    model/framework/laptop-13-amd-ryzen-7040

    processor/amd/ryzen-7-7840u
    graphics/amd/radeon-780m
    wifi/realtek/rtl8852be

    quirk/framework/linux-7-2-dcn-freeze

IDs are API.

Do not make them depend on:

- temporary filesystem layout;
- GjallarOS profile paths;
- Nix module paths;
- device enumeration order;
- kernel sysfs paths;
- PCI bus position;
- current hostname.

Renaming a directory must not silently change hardware identity.

---

## 5. Canonical catalog shape

The target catalog is an entity registry.

An entity's ID is its address. The entity `wifi/realtek/rtl8852be` lives in
`catalog/entities/wifi/realtek/rtl8852be.json`, and nowhere else:

    catalog/entities/
    ├── class/laptop.json
    ├── vendor/framework.json
    ├── family/framework/laptop-13.json
    ├── model/framework/laptop-13-amd-ryzen-7040.json
    ├── wifi/realtek/rtl8852be.json
    └── quirk/framework/fprintd-resume.json

Validation refuses an entity stored anywhere else. So a client that knows an
ID fetches exactly that entity, without reading the rest of the catalog.

A model should contain references rather than copies.

Example:

    {
      "apiVersion": "oddc.openjade.de/v2",
      "kind": "DeviceModel",

      "metadata": {
        "id": "model/framework/laptop-13-amd-ryzen-7040",
        "vendorRef": "vendor/framework",
        "familyRef": "family/framework/laptop-13"
      },

      "identity": {
        "dmi": {
          "systemVendor": "Framework",
          "productName": "Laptop 13 (AMD Ryzen 7040Series)",
          "boardProduct": "FRANMDCP07"
        }
      },

      "hardware": {
        "processor": {
          "primary": {
            "ref": "processor/amd/ryzen-7-7840u"
          }
        },

        "graphics": {
          "integrated": {
            "ref": "graphics/amd/radeon-780m"
          }
        },

        "network": {
          "wifi": {
            "primary": {
              "ref": "wifi/realtek/rtl8852be"
            }
          }
        }
      },

      "quirks": {
        "linux-7-2-dcn-freeze": {
          "ref": "quirk/framework/linux-7-2-dcn-freeze",
          "enabled": true
        }
      }
    }

The processor definition belongs to the processor entity.

The device model only says that this machine contains that processor.

---

## 6. When to create a component entity

Not every leaf value deserves its own JSON file.

Create a reusable component entity when at least one of these is true:

- the component appears in multiple device models;
- the component has its own stable hardware identity;
- the component carries reusable Linux knowledge;
- the component has substantial metadata;
- the component may independently gain evidence, quirks, or support information.

Examples that normally deserve entities:

- CPU models;
- GPUs;
- Wi-Fi chipsets;
- Bluetooth controllers;
- fingerprint readers;
- touch controllers;
- storage controllers.

Examples that usually stay inline:

- number of speakers;
- whether a chassis has a webcam;
- a model-specific thermal threshold;
- a DMI product string.

Do not normalize data merely for aesthetic purity.

Normalize it when doing so gives the information a clear reusable owner.

---

## 7. Canonical versus resolved views

Canonical data should be normalized.

Consumer data should be convenient.

ODDC therefore exposes a **resolved view**.

The resolver follows references and produces a hierarchical object suitable for programs and Nix.

For example, a consumer may query:

    oddc.resolved.vendor.name

    oddc.resolved.model.name

    oddc.resolved.hardware.processor.primary.model.name

    oddc.resolved.hardware.processor.primary.vendor.name

    oddc.resolved.hardware.graphics.integrated.model.name

    oddc.resolved.hardware.network.wifi.primary.driver

    oddc.resolved.policy.thermal.fanControl.thermalEnterC

This is intentionally easy to read.

The value may originate in another canonical entity, but the caller does not need to manually traverse the registry.

The resolved object is generated.

It is never another editable source of truth.

---

## 8. Nix interface

ODDC's public Nix interface consumes the resolved catalog.

It must not duplicate hardware values in per-device Nix modules.

Target usage:

    {
      imports = [
        inputs.oddc.nixosModules.default
      ];

      oddc.device = "model/framework/laptop-13-amd-ryzen-7040";
    }

The module exposes read-only resolved hardware information and configurable policy.

Conceptually:

    config.oddc.resolved.hardware.processor.primary.model.name

    config.oddc.resolved.hardware.graphics.integrated.driver

    config.oddc.resolved.policy.thermal.fanControl.thermalEnterC

A host override should be easy:

    oddc.overrides.policy.thermal.fanControl.thermalEnterC = 85;

The canonical catalog remains unchanged.

The final resolved value becomes 85.

---

## 9. No per-device Nix duplication

A normal new laptop must not require a new Nix module.

Avoid:

    nixos/modules/devices/framework-laptop-13-amd-ryzen-7040.nix
    nixos/modules/devices/hp-zbook-x2-g4.nix
    nixos/modules/devices/every-future-machine.nix

Instead there is one generic catalog consumer.

Generic capability modules implement reusable behavior:

    nixos/modules/
    ├── default.nix
    ├── deployment.nix
    ├── public-interface.nix
    ├── capabilities/
    │   ├── alps-touchpad.nix
    │   ├── fw-fanctrl.nix
    │   ├── orientation.nix
    │   ├── quick-keys.nix
    │   ├── thermald.nix
    │   └── tablet-mode.nix
    └── quirks/
        ├── fprintd.nix
        ├── kernel-fallback.nix
        ├── pch-pme-sci.nix
        └── runtime-power.nix

The catalog determines which capabilities and quirks apply.

### Which module reads what

Each module reads only these paths of `config.oddc.resolved`. The owner is
the entity that holds the value in the current catalog; a module never
reads a vendor or model ID to decide what to do.

| Module | Resolved path it reads | Owner |
| --- | --- | --- |
| `default.nix` | `policy.kernel.parameters` | the device model |
| `default.nix` | `class.policy.power.powerProfilesDaemon.enable` | `class/laptop` |
| `capabilities/fw-fanctrl.nix` | `policy.thermal.fanControl` | the device model |
| `quirks/fprintd.nix` | `quirks.*.fprintd`, `quirks.*.enabled` | the quirk entity; `enabled` the device model |
| `quirks/fprintd.nix` | `hardware.security.fingerprint.primary.{bus,deviceId}` | the fingerprint component entity |
| `quirks/kernel-fallback.nix` | `quirks.*.affected.kernel.{minimum,maximumBefore}`, `quirks.*.fallbackPackage`, `quirks.*.enabled` | the quirk entity; `enabled` the device model |
| `quirks/pch-pme-sci.nix` | `quirks.*.pchPmeSci.{action,rootPorts}`, `quirks.*.enabled` | the quirk entity; `enabled` the device model |
| `quirks/runtime-power.nix` | `quirks.*.runtimePower`, `quirks.*.enabled` | the quirk entity; `enabled` the device model |
| `capabilities/alps-touchpad.nix` | `hardware.input.keyboard.primary.{deviceId,bluetooth.deviceId,touchpad}` (`touchpad.protocol` `alps-u1-report-passthrough`) | the keyboard component entity |
| `capabilities/orientation.nix` | runs only with `policy.sensors.orientation.service` (`iio-sensor-proxy`); checks `hardware.sensors.hub.provides.orientation`, also exposed read-only as `oddc.sensors.orientation.present` | the device model |
| `capabilities/quick-keys.nix` | `hardware.input.quickKeys.primary.{deviceId,access.mappable,protocol}` | the Quick Keys component entity |
| `capabilities/quick-keys.nix` | `policy.input.quickKeys.{keymap,presetKeys}` | the device model |
| `capabilities/thermald.nix` | `policy.thermal.daemon` (`thermald`) | the device model |
| `capabilities/tablet-mode.nix` | runs only with `policy.input.tabletMode.from` (`keyboard-presence`); reads `hardware.input.keyboard.*.{attachment,detachSignal,bus,deviceId}`, `capabilities.{tabletModeSwitch,kickstandSwitch}` | the device model; `bus`, `deviceId` the keyboard component entity |
| `deployment.nix` | the whole resolved view, written to `/etc/oddc/resolved.json` | each value's own owner |

Policy no module here reads, such as `vendor.policy.secureBoot` or
`policy.graphics.discrete.driverBranch`, is for consumers: they read it
from `config.oddc.resolved` or `/etc/oddc/resolved.json`, and fail closed
when it is absent.

A module that reads a new path adds its row here in the same change.

---

## 10. Quirks

A quirk is exceptional behavior required for a specific hardware/software combination.

Quirks have two distinct owners:

### Quirk metadata

Canonical catalog data describes:

- stable quirk ID;
- affected hardware;
- affected kernel or software range;
- reason;
- source;
- removal condition;
- whether it is enabled for a model.

### Quirk implementation

Executable Nix or program logic lives in exactly one implementation module.

Example:

    catalog/entities/quirk/framework/linux-7-2-dcn-freeze.json

and the generic module that acts on what it declares:

    nixos/modules/quirks/kernel-fallback.nix

The module acts on the quirk's resolved values, not on its ID, so another
quirk of the same kind needs no new Nix.

The device model references the quirk ID.

It must not copy the implementation or the quirk metadata.

### Shared quirks

A quirk is one behaviour, not one machine. When several models need the
same behaviour, they reference the same quirk entity; nobody writes a second
entity with the same data. The quirk holds only what is the same for all of
them. What differs per machine (which GPU, which port) stays a fact of the
model or its components, and the implementation reads it from there.

Sharing a quirk shares the definition, never the proof: each model that
enables it needs its own evidence with `quirks.KEY=pass`. Facts never switch
a quirk on; only the model's reference does.

A quirk may name the resolved paths it needs, each under a name of its
own (paths are values, never keys: a key with dots would read as a path of
the model once the quirk is enabled):

    "requires": {
      "discreteDriver": "hardware.graphics.discrete.driver",
      "integratedDriver": "hardware.graphics.integrated.driver"
    }

`oddc quirk enable QUIRK` refuses a model that lacks one of them, adds the
reference and prints the evidence step. `oddc quirk share` merges quirk
entities that hold the same data.

`mkForce` is permitted only where a documented safety or compatibility boundary genuinely requires it.

---

## 11. Policy is not hardware fact

Separate physical facts from recommended configuration.

Bad:

    "cpu": {
      "name": "AMD Ryzen 7 7840U",
      "quietFanTemperature": 60
    }

Better:

    "hardware": {
      "processor": {
        "primary": {
          "ref": "processor/amd/ryzen-7-7840u"
        }
      }
    },

    "policy": {
      "thermal": {
        "fanControl": {
          "quietEnterC": 60
        }
      }
    }

This matters because policy may evolve while the hardware fact remains permanently true.

---

## 12. Evidence

Evidence records what happened on real hardware.

Evidence is not a second catalog.

Example:

    evidence/
    └── model/framework/laptop-13-amd-ryzen-7040/
        ├── 2026-09-15.json
        └── 2027-02-04.json

Evidence lives below the ID of the model it is about.

Evidence should contain:

- device model ID;
- ODDC revision;
- validation timestamp;
- observed component matches;
- capability checks;
- failures and warnings;
- relevant software/kernel versions.

Evidence must avoid unnecessary private identifiers such as:

- serial numbers;
- MAC addresses;
- user names;
- machine hostnames;
- filesystem UUIDs.

Evidence is append-only historical material.

Do not rewrite old evidence to match today's catalog.

---

## 13. Overrides

Overrides are explicit layers outside canonical hardware knowledge. They
belong to the consumer's configuration (`oddc.overrides`), never to the
catalog: inside the catalog nothing overrides anything.

Precedence:

    canonical catalog
      -> project override
        -> host override

Overrides must use stable named paths.

Good:

    policy.thermal.fanControl.thermalEnterC

Bad:

    thermal.curves[2].values[4]

Overrideable collections should therefore use keyed objects rather than positional arrays whenever practical.

Each resolved leaf should retain provenance so tooling can answer:

    oddc explain \
      --device model/framework/laptop-13-amd-ryzen-7040 \
      --path policy.thermal.fanControl.thermalEnterC

and report where the final value came from.

---

## 14. GjallarOS is a consumer

ODDC must not encode GjallarOS implementation details into portable catalog data.

Forbidden canonical data includes:

- GjallarOS profile paths;
- GjallarOS-generated settings paths;
- installer-specific directory names;
- recovery implementation paths;
- GjallarOS-only service names.

GjallarOS consumes the same ODDC API that another NixOS project can consume.

GjallarOS consumes canonical ODDC model IDs directly.

Compatibility maps, legacy runtime profile IDs, and parallel hardware ownership
trees are not part of the ODDC v2 architecture and must not be reintroduced.

---

## 15. Generated files

Generated representations must not become editable sources.

Examples:

- resolved catalog snapshots;
- fw-fanctrl configuration;
- generated Nix data;
- installer caches;
- recovery snapshots;
- compatibility exports.

If a generated representation is persisted for recovery or auditing, it must record the canonical ODDC revision from which it was produced.

Do not edit the generated snapshot to change canonical behavior.

Change its canonical owner and regenerate.

---

## 16. Adding a new device

The normal workflow should be:

### 1. Identify the model

Create one device model entity.

### 2. Reference existing components

Search the catalog first.

If the processor, GPU, Wi-Fi chipset, or another component already exists, reference it.

Do not duplicate it.

### 3. Add genuinely new components

Only add a component entity when the catalog does not already have the component.

### 4. Add model-specific policy

Only values actually specific to this model belong here.

### 5. Reference existing quirks

If a reusable quirk already exists, reference it.

### 6. Add new implementation only when required

A normal device addition should require no new Nix.

New Nix is justified only when hardware requires genuinely new behavior.

### 7. Validate real hardware

Add a sanitized evidence record.

---

## 17. Expected contribution size

For an ordinary newly supported laptop, the ideal contribution is approximately:

    1 device model JSON
    1 evidence JSON
    0 Nix files
    0 test files

Tests run over the real catalog and take their expectations from it and from
evidence: every model must resolve, keep one owner per value, match its own
DMI identity, deploy alone and pass `oddc doctor` on its own hardware. A new
model is tested by being in the catalog. Tests never repeat device facts and
never use invented devices; tests of rejected input break a temporary copy of
the real catalog.

Go tests live under `tests/`, one folder per package they test, and reach the
catalog through `tests/fixture`. A new `oddc` command goes in the
`internal/` folder for its use (`system`, `catalog` or `contribute`) and is
listed in `cmd/oddc`; keep each Go file to one concern, under about 200 lines.

Additional component entities are added only for hardware not already represented.

Additional Nix is exceptional.

If adding one laptop requires six device-specific Nix and JSON files, stop and reconsider the model.

---

## 18. Forbidden duplication

`oddc validate`, and so CI, rejects these already:

- two quirk entities with the same data;
- quirk data written into a model next to its reference;
- a device (bus and deviceId) described inside a model instead of referenced;
- two component entities claiming the same bus and deviceId.

CI should eventually reject these patterns too:

- the same PCI or USB component facts copied into multiple models;
- canonical fan policy copied into a runtime profile;
- canonical Secure Boot policy copied into a GjallarOS manifest;
- per-device Nix modules containing catalog values;
- editable generated fw-fanctrl configuration;
- legacy runtime profile IDs inside portable catalog entities;
- duplicated component metadata embedded in models instead of referenced;
- committed resolved snapshots treated as canonical source;
- device facts repeated in Go or Nix tests, or invented test devices.

---

## 19. Source provenance

Important hardware claims should have source provenance.

Useful source classes include:

- vendor documentation;
- kernel documentation;
- upstream driver source;
- fwupd metadata;
- linux-hardware / hw-probe style observations;
- ODDC real-machine evidence.

A source proves a claim.

It does not become another place where the claim is independently maintained.

---

## 20. Naming style

Canonical JSON uses lower camelCase for fields:

    processorName
    productName
    thermalEnterC
    fanControl

Stable IDs use lowercase kebab-case segments:

    model/framework/laptop-13-amd-ryzen-7040
    processor/amd/ryzen-7-7840u

Nix exposes hierarchical attribute paths:

    oddc.resolved.hardware.processor.primary.model.name
    oddc.resolved.policy.thermal.fanControl.thermalEnterC

Prefer meaningful names over abbreviations.

---

## 21. Review questions

Before accepting an ODDC change, ask:

1. Who owns every new fact?
2. Does the same fact already exist elsewhere?
3. Could this be a reference instead of a copy?
4. Is this hardware fact, policy, evidence, or implementation?
5. Is a new Nix module actually necessary?
6. Is this path stable enough to become public API?
7. Can another operating system or NixOS project consume this without knowing GjallarOS?
8. Can `oddc explain` identify why the final value exists?
9. Would changing this value require editing more than one authoritative file?

If question 9 is yes, the design probably violates the single-source-of-truth rule.

---

## 22. Framework Laptop 13 AMD 7040 example

The Framework 7040 currently serves as the main migration example.

The desired end state is approximately:

    vendor/framework
        owns Framework-wide facts and policies

    family/framework/laptop-13
        owns Laptop 13 family/chassis facts

    model/framework/laptop-13-amd-ryzen-7040
        owns DMI identity, composition, and model-specific policy

    processor/amd/ryzen-7-7840u
        owns processor facts

    graphics/amd/radeon-780m
        owns graphics facts

    wifi/realtek/rtl8852be
        owns Wi-Fi chipset facts

    quirk/framework/linux-7-2-dcn-freeze
        owns compatibility metadata

    nixos/modules/quirks/kernel-fallback.nix
        owns the executable workaround

    evidence/...
        records what real Framework machines actually validated

There must not also be a second editable Framework runtime profile containing those same values.

---

## 23. End goal

ODDC should behave like a hardware library, not a profile warehouse.

Canonical storage is normalized and unambiguous.

Consumers receive a convenient hierarchical resolved view.

Hardware support should become easier as the catalog grows because new models increasingly reuse existing entities.

The design succeeds when:

> adding support for another machine mostly means describing what hardware it contains, not writing another operating-system configuration.

## Optional policy presence semantics

Do not add negative placeholder policy objects merely to record that a feature
is unsupported.

For Secure Boot firmware ownership specifically:

- absence of `policy.secureBoot` means ODDC has no trusted supported policy for
  that entity;
- presence of `policy.secureBoot` means support is explicitly known and the
  operational policy must be complete and validated;
- do not add `supported: true` or `supported: false` fields to canonical
  Secure Boot policy data.

Consumers must fail closed when the policy is absent.

This keeps canonical data declarative: it records knowledge that exists rather
than maintaining duplicated negative capability flags.

## USB identity of a part

A USB component may say what tells one unit from another, measured
read-only on a real unit (`tools/usb-identity-probe.py`, a contributor tool: string descriptors, the
Microsoft OS string, BOS, HID feature reports; values never recorded):

    "usbIdentity": { "serial": "none", "anchor": "bluetooth-address" }

`serial` is `per-unit`, `placeholder` (a template every unit carries, such
as `ABCD123456789`) or `none`; `anchor` names a per-unit identity beyond USB.
Leave it out until it is measured. Trust brokers use it to decide whether a
changed serial means a swapped part.

## The toolchain follows the same rules

The `oddc` commands are held to the ideology like the catalog is. This is
how fwupd (one daemon, protocol plugins, device data in quirk files) and
systemd's hwdb (generic tools, device data keyed by IDs) keep device
knowledge out of code.

1. **One inventory, many views.** `oddc devices` is the one list of a
   model's parts: placement, attachment, fixed USB port, Linux support,
   what the evidence proves, protocol. Tools and consumers filter it; none
   walks the resolved model for its own view, and there is no command per
   view (a USB trust broker takes the devices with a `usbPort`).
2. **No device knowledge in code.** A command never names a vendor, model or
   device ID. What a device needs lives in its entity.
3. **Behaviour only by explicit reference.** A command acts on a model only
   when the model references what it implements (a quirk, a policy, a
   component's `protocol`). It never detects hardware and decides by
   itself; without the reference it refuses with an error.
4. **Protocol tools for the edge cases.** When a device needs code, the
   command implements the protocol, not the device (`oddc touchpad` speaks
   `alps-u1-report-passthrough` to any component that names it). It finds
   its component through the inventory and takes `--device <path>` to aim
   at one when a model has several.
5. **Proven by evidence.** Every behaviour a command brings to a model has
   its result in that model's evidence (the component path, `quirks.KEY`,
   `policy.KEY`).
6. **Fail closed.** Missing or contradictory data is an error, never a
   guess or a fallback.
7. **Debug tools are labelled and read-only by default.** Probes and guided
   tests read the same inventory and say they are for contributors. Nothing
   writes to a device (firmware, ownership, settings) without the owner's
   explicit decision.

## Planned work and tips

A model may say what is planned for it and what an owner should know, in
plain words for owners, under `support`:

    "support": {
      "planned": {
        "fingerprintDriver": {
          "summary": "An open Linux driver for the fingerprint reader.",
          "item": "ODDC-96"
        }
      },
      "notes": {
        "keyboardReconnect": { "summary": "..." }
      }
    }

Every entry needs a `summary`; `item` names the work item. Plan only what
has an open item, and drop the entry when the item is done or dropped.
`oddc notes` and the release pull request show them next to what the
evidence proves works and what does not.

## Security findings

The short how-to is [docs/SECURITY.md](docs/SECURITY.md); this section has
the rules.

A security weakness of hardware or its firmware is a `Security` entity
below `catalog/entities/security/<area>/<name>.json`. It is the one owner of
everything known about that weakness. A model or component references it
under its own `security` section and holds nothing about it itself:

    "security": {
      "thunderboltDma": {
        "ref": "security/thunderbolt/no-kernel-dma-protection"
      }
    }

Put the reference where the weakness lives. A weakness of a chip goes on
that chip's component entity, so every model that uses the chip carries it
with one edit. A weakness of one firmware goes on the model. Facts never
attach a finding; only a reference does. Below `security/`, only `Security`
entities live.

The entity holds:

- `summary`: one or two plain sentences for users, at most 240 characters:
  what it means for them and what to be careful about. A contributor who
  reports a finding writes it, and review checks that it informs without
  frightening;
- `details`: `what`, `attack` and `fix` are required, plus `when`, `check`
  and `references` where known. This is the full account for contributors;
- `severity`: `low`, `medium`, `high` or `critical`;
- `status`: `open`, `mitigated` (the details name the fix) or `withdrawn`
  (it turned out not to hold; it stays so references and history remain
  readable);
- `reported` and `modified`: dates. Every change updates `modified`;
- `fwupdHsi`: optional, fwupd host security attributes whose failure
  confirms the finding, each under a name of its own (the catalog holds no
  lists). fwupd is the established tool for these checks, so a round
  confirms the finding with it instead of code of its own;
- `flags`: named properties the finding takes away, always `false`.

Rules:

- **Findings only lower trust.** A flag is false as soon as one active
  finding says so. A flag no finding names is unknown, and consumers treat
  unknown as unprotected. A claim that something is protected needs its
  own per-model policy and evidence, never a finding.
- **They take effect at once.** A reported finding reaches every install that
  follows the catalog with its next update, without waiting for a release of
  the operating system that consumes it.
- **The proof is per model.** A finding changes the model's closure, so the
  model shows as changed until its owner records evidence with
  `security.<key>=confirmed` (by the reference's path), checked on the
  machine with the finding's `check`.
- **No override touches a finding.** A project or host override that sets,
  changes or deletes one, or deletes a node that holds one, is refused:
  findings change only in the catalog, by reviewed pull request.
- **Flag names are an interface.** Consumers act on them, so a flag is never
  renamed or reused for another meaning; a new meaning gets a new name. A
  consumer that no longer finds a flag treats it as unknown, so a lost flag
  fails closed, never open.
- **Live facts only lower trust too.** When a consumer can read a flag live
  (for example the kernel's `iommu_dma_protection`) and the live value is
  worse than the catalog's, it shows the worse one and reports the
  contradiction as a bug against the model. A live fact never grants
  protection on its own.

`oddc security` gives users a calm notice: each summary, and that the ODDC
team knows and nothing needs doing. `oddc security --details` is the full
account for contributors, and `oddc security --json` is for programs (the
findings and the combined flags). `oddc doctor` shows the same notice.

## ODDC versus operating-system policy

ODDC must not become a dumping ground for general GjallarOS laptop behavior.

A setting belongs in ODDC only when it is hardware-specific knowledge,
hardware-specific policy, or the activation of a reusable hardware
implementation.

Examples that belong in GjallarOS rather than ODDC:

- generic low-battery warnings;
- generic emergency shutdown thresholds;
- generic laptop lid behavior;
- generic boot-loader presentation;
- operating-system-wide laptop power-management defaults.

Examples that belong in ODDC:

- a stable hardware identity;
- a model-specific kernel quirk;
- a device-specific runtime power workaround;
- a validated fan policy for a known embedded controller;
- hardware-specific driver selection.

The canonical graph describes the hardware-specific condition or policy.
Reusable Nix modules implement it.


## Licence and copyright

ODDC belongs to everyone who works on it. No person or company holds it.

- Every file carries only `SPDX-License-Identifier: GPL-3.0-or-later`. Do not
  add a personal `Copyright (C) <name>` line, author tag or maintainer list.
- Where a format needs a copyright line, for example a driver sent to an
  upstream project, it reads `Copyright (C) <year> The ODDC contributors`.
- There is no contributor licence agreement. Each contribution stays under
  GPL-3.0-or-later from its author, so no one can relicense the collection or
  take it closed on their own.
