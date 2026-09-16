# ODDC overrides

ODDC supports small project and host override documents.

Resolution order:

    class
    vendor
    family
    device
    project
    host

An override only declares the leaves it changes.

Example:

    {
      "schemaVersion": "2.0.0",
      "id": "host/example",
      "kind": "host",
      "targetDevice": "framework-laptop-13-amd-ryzen-7040",
      "blocks": {
        "thermal": {
          "fan-control": {
            "policy": {
              "thermalEnterC": 85
            }
          }
        }
      }
    }

The rest of the device definition remains inherited.

Stable block paths are part of the ODDC public interface.
