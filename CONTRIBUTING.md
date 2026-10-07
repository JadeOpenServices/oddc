# Contributing to ODDC

ODDC is a portable hardware knowledge base.

It is not a collection of duplicated Nix profiles.

The central design rule is:

> Every hardware fact, policy value, and implementation behavior has one authoritative owner.

Everything else must reference, resolve, generate, or observe that authoritative value.

This document defines how ODDC data is structured, how contributors add hardware, and which forms of duplication are forbidden.

---

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

Inheritance or defaults may still be used where there is a genuine semantic default, but it must not be used as a substitute for explicit hardware relationships.

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

    nixos/
    ├── default.nix
    ├── capabilities/
    │   ├── fan-control.nix
    │   ├── battery.nix
    │   ├── graphics.nix
    │   └── secure-boot.nix
    └── quirks/

The catalog determines which capabilities and quirks apply.

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

    catalog/quirks/framework/linux-7-2-dcn-freeze.json

and:

    nixos/quirks/framework-linux-7-2-dcn-freeze.nix

The device model references the quirk ID.

It must not copy the implementation or the quirk metadata.

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

Overrides are explicit layers outside canonical hardware knowledge.

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

CI should eventually reject these patterns:

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

    nixos/quirks/framework-linux-7-2-dcn-freeze.nix
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
