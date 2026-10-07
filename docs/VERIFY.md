# Verifying a model

A model is supported only once it was proven on the real machine. That
proof is passing evidence: a record below `evidence/<model>/` with the
status `runtime-verified` or `hardware-validated`, made on the model's
current closure. `oddc status` tells whether a model is `verified`,
`changed` or `unverified`; see [WORKFLOW.md](WORKFLOW.md#the-oddc-command).

## What each status proves

- `documented`: written from documentation; proves nothing.
- `detected`: the machine was seen; proves nothing.
- `runtime-verified`: the machine ran a deployment of the model. Its
  identity matched (`identity: pass`) and every driver the catalog names
  for a component was bound to it.
- `hardware-validated`: runtime-verified, and every part of the model was
  tested by hand and works as the catalog says.

The last two are written only when the record proves them, and the
validator rejects passing evidence on the current closure that does not.

## The results hardware-validated needs

One result per part of the resolved model, named by its path:

- `identity`
- every component, as `hardware.network.wifi.primary`: it works.
- every capability, as `capabilities.usb4`: it works. A capability the
  catalog sets to `false` must be `not-exposed`: the machine really lacks it.
- every quirk, as `quirks.fprintdResume`: the problem it works around does
  not occur with it.
- every policy section, as `policy.thermal`: the system behaves as it says.

Every one of them is `pass` unless named otherwise above. You do not have
to look them up: `oddc evidence record --status hardware-validated` lists
each one missing and writes nothing until none is.

## Steps

1. Deploy the model on the machine from the ODDC revision under test, so
   `/etc/oddc` holds it. Use a checkout of that same revision for the
   steps below; recording refuses when the deployed model is not the one
   in the checkout.
2. `oddc doctor` must pass.
3. Test each part, then record:

       oddc evidence record --status hardware-validated \
         --result hardware.network.wifi.primary=pass \
         --result capabilities.chargeThresholds=not-exposed \
         ...

   The record holds the OS, kernel, BIOS, the ODDC revision and closure of
   the deployment and the drivers bound to each component; nothing that
   identifies the machine or you.
4. `oddc status` must say `verified`.
5. Send it with `oddc contribute`.

If anything fails, record what was tested with `--status detected` and the
failing results, and fix the catalog before trying again. Any later change
to the model's closure makes it `changed`, and it must be proven again.
