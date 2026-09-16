# ODDC overrides

Canonical ODDC entities describe the shared hardware model.

Projects and individual hosts may change policy without modifying canonical
hardware knowledge.

Resolution order is:

    canonical entities
      -> project overrides
        -> host overrides

An override targets a stable canonical model ID.

Example project override:

    {
      "$schema": "../schemas/overlay.schema.json",
      "apiVersion": "oddc.openjade.de/v2",
      "id": "project/example",
      "kind": "project",
      "targetModel": "model/framework/laptop-13-amd-ryzen-7040",
      "overrides": {
        "policy": {
          "thermal": {
            "fanControl": {
              "policy": {
                "thermalEnterC": 85
              }
            }
          }
        }
      }
    }

A host override uses the same structure with:

    "kind": "host"

and has higher precedence than project overrides.

Overrideable collections use named objects rather than positional arrays.
This keeps paths stable and explainable.

Use `oddcctl explain` to inspect the final owner and history:

    oddcctl explain \
      --device model/framework/laptop-13-amd-ryzen-7040 \
      --project ./project.json \
      --host ./host.json \
      --path policy.thermal.fanControl.policy.thermalEnterC

Canonical entity data never depends on or references a downstream project or
host override.
