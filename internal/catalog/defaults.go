// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import (
	"path/filepath"

	"github.com/JadeOpenServices/oddc/internal/cli"
)

// WithDefaults completes args for a deployed system: the root defaults to
// the system root when it holds a deployment, else the working directory;
// resolve, explain and status default to the deployed model, and resolve
// and explain to its host overlay.
func WithDefaults(
	args []string,
	system string,
) []string {
	result := append([]string{}, args...)

	if !cli.Has(result, "--root") {
		root := "."
		if cli.DeployedModel(system) != "" {
			root = system
		}

		result = append(result, "--root", root)
	}

	if args[0] != "resolve" && args[0] != "explain" && args[0] != "status" {
		return result
	}

	root := cli.Value(result, "--root", ".")
	model := cli.DeployedModel(root)

	if !cli.Has(result, "--device") && model != "" {
		result = append(result, "--device", model)
	}

	overlay := filepath.Join(root, "host-overlay.json")
	if !cli.Has(result, "--host") &&
		cli.Value(result, "--device", "") == model &&
		cli.Exists(overlay) {
		result = append(result, "--host", overlay)
	}

	return result
}
