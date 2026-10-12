# Security findings

A security finding is a known weakness of a piece of hardware or its
firmware, such as "this machine's Thunderbolt port has no DMA protection".
ODDC keeps each one in its own entity below `catalog/entities/security/`.
Models and components only point at it. The rules behind this are in
[CONTRIBUTING.md](../CONTRIBUTING.md#security-findings); this page is the
short version.

## What users see

    $ oddc security
    Known security findings for this model: 1. The ODDC team is on them; nothing for you to do.
    - Thunderbolt without DMA protection: A Thunderbolt device you approve can read and write all of the computer's memory. ...
    Contributors: `oddc security --details` shows the full account.

`oddc doctor` shows the same notice. `oddc security --details` shows
everything: what, attack, severity, fix, how to check it.

## Report a finding

Only report what you measured on a real machine. A guess or a "could be"
is not a finding yet.

1. Check whether an entity for it already exists: `ls catalog/entities/security/*`.
   If one does, reference it (step 3) and skip step 2.
2. Create `catalog/entities/security/<area>/<name>.json`, for example
   `security/thunderbolt/no-kernel-dma-protection.json`:

       {
         "$schema": "../../../../schemas/entity.schema.json",
         "apiVersion": "oddc.openjade.de/v2",
         "kind": "Security",
         "metadata": { "id": "security/<area>/<name>", "name": "Short title" },
         "data": {
           "summary": "What it means for users and what to be careful about. Plain, calm, at most 240 characters.",
           "severity": "medium",
           "status": "open",
           "reported": "2026-10-12",
           "modified": "2026-10-12",
           "details": {
             "what": "What is wrong, as measured.",
             "attack": "Who can do what with it, and what they need (physical access, ...).",
             "fix": "What fixes or reduces it, and what is still open.",
             "check": "The command that shows it on a machine.",
             "references": "Links: CVE, vendor advisory, fwupd HSI attribute, ..."
           },
           "flags": { "thunderboltDmaProtected": false }
         },
         "sources": {
           "<machine>Measured": { "type": "validation", "observedAt": "2026-10-12", "reason": "What you measured, on which machine, BIOS and kernel." }
         }
       }

   - `severity`: `low`, `medium`, `high` or `critical`.
   - `status`: `open`, `mitigated` (the details name the fix) or
     `withdrawn` (it turned out wrong; never delete the file).
   - `flags`: what the finding takes away, always `false`. Reuse an existing
     flag name if it means the same thing; never rename one.
   - Every later change updates `modified`.
3. Reference it where the weakness lives. A chip's weakness goes on the
   chip's component entity, so every model with that chip gets it. A
   weakness of one model's firmware goes on the model:

       "security": {
         "thunderboltDma": { "ref": "security/thunderbolt/no-kernel-dma-protection" }
       }

4. `oddc validate` must pass. It checks the summary, the details, severity,
   status, dates and that every flag is `false`.
5. Open the pull request. Put the `summary` in its description: reviewers
   check that it informs without frightening.

## Afterwards

Every model that references the finding now shows as `changed` in
`oddc status`. Its owner checks the finding on the machine (the `check`
command) and records new evidence with the result `security.<key>=confirmed`
(the reference's path, for example `security.thunderboltDma`); see
[VERIFY.md](VERIFY.md). Until then the finding already applies: a finding
only lowers trust, so it never waits for proof.

## For consumers

Read `oddc security --json`: the active findings and the combined `flags`.
A flag that is not in the map is unknown, and unknown means unprotected.
Never keep your own per-model copy of a flag: a new finding then reaches
your users with the next ODDC update. See [CONTRACT.md](CONTRACT.md).
