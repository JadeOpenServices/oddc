// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import (
	"errors"
	"fmt"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunNotes prints a model's entry for the people who own the machine
// (--device ID, default the deployed model; --root DIR, default the
// deployment): what works, what does not yet, security, plans, tips.
func RunNotes(args []string) error {
	root := cli.Value(args, "--root", cli.SystemRoot)
	model := cli.Value(args, "--device", cli.DeployedModel(root))
	if model == "" {
		return errors.New("name the model with --device")
	}
	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return err
	}
	notes, err := registry.DeviceNotes(model)
	if err != nil {
		return err
	}
	fmt.Print(notes)
	return nil
}
