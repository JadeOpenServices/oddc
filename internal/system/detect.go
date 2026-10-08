// SPDX-License-Identifier: GPL-3.0-or-later

// Package system holds the commands for this machine and its system flake:
// detect, setup, fetch, doctor and update.
package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

func describe(identity oddc.MachineIdentity) string {
	return fmt.Sprintf(
		"vendor %q, product %q, board %q, form factor %q",
		identity.SysVendor,
		identity.ProductName,
		identity.BoardName,
		identity.FormFactor,
	)
}

func noMatch(identity oddc.MachineIdentity) error {
	return fmt.Errorf(
		"no ODDC model matches this machine (%s); "+
			"add it with `oddc scaffold` and `oddc contribute`",
		describe(identity),
	)
}

// Detect fetches this machine's model and loads it.
func Detect(args []string) (*oddc.Registry, string, error) {
	dir, err := os.MkdirTemp("", "oddc-Detect-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(dir)

	src, err := cli.Source(args, dir)
	if err != nil {
		return nil, "", err
	}

	identity := oddc.ReadIdentity(cli.Value(args, "--sys", "/sys"))
	answer := filepath.Join(dir, "answer")

	model, err := oddc.Fetch(src, identity, answer)
	if errors.Is(err, oddc.ErrNoModelMatch) {
		return nil, "", noMatch(identity)
	}
	if err != nil {
		return nil, "", err
	}

	registry, err := oddc.LoadRegistry(answer)
	if err != nil {
		return nil, "", err
	}

	return registry, model, nil
}

// RunFetch writes only this machine's model, or the --device model, to
// --out: its reference closure, its evidence and the ODDC revision.
func RunFetch(args []string) error {
	out := cli.Value(args, "--out", "")
	if out == "" {
		return errors.New("--out is required")
	}

	scratch, err := os.MkdirTemp("", "oddc-fetch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)

	src, err := cli.Source(args, scratch)
	if err != nil {
		return err
	}

	model := cli.Value(args, "--device", "")
	if model != "" {
		err = oddc.FetchModel(src, model, out)
	} else {
		identity := oddc.ReadIdentity(cli.Value(args, "--sys", "/sys"))

		model, err = oddc.Fetch(src, identity, out)
		if errors.Is(err, oddc.ErrNoModelMatch) {
			return noMatch(identity)
		}
	}
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", model, src.Revision())
	return nil
}

func RunDetect(args []string) error {
	registry, model, err := Detect(args)
	if err != nil {
		return err
	}

	fmt.Printf(
		"%s\t%s\n",
		model,
		registry.Entities[model].Metadata.Name,
	)

	return nil
}
