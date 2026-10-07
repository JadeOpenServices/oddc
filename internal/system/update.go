// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"fmt"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
)

// RunUpdate takes the newest catalog of the stage a system flake follows
// into it, after --stage switches the stage, and with --switch rebuilds.
func RunUpdate(args []string) error {
	flake := cli.Value(args, "--flake", "/etc/nixos")

	untracked := Untracked(flake)
	if len(untracked) > 0 && cli.Has(args, "--switch") {
		return fmt.Errorf(
			"%s has files git does not track (%s), which nixos-rebuild --flake leaves out; rebuild it as it is meant to be built",
			flake, strings.Join(untracked, ", "),
		)
	}

	before, err := LockedOddc(flake)
	if err != nil {
		return err
	}

	if name := cli.Value(args, "--stage", ""); name != "" {
		url, ok := Stages[name]
		if !ok {
			return fmt.Errorf("--stage is main or staging, not %q", name)
		}

		if before.Stage() == "" {
			return fmt.Errorf(
				"the oddc input of %s is not %s; --stage switches only that",
				flake, cli.Upstream,
			)
		}

		if err := SetStage(flake, url); err != nil {
			return err
		}
	}

	if err := cli.Command(
		"nix", "flake", "update", "oddc", "--flake", flake,
	); err != nil {
		return err
	}

	after, err := LockedOddc(flake)
	if err != nil {
		return err
	}

	from, to := short(before.Locked.Rev), short(after.Locked.Rev)
	switch {
	case before.Stage() != after.Stage():
		fmt.Printf("oddc: %s %s -> %s %s\n", before.Stage(), from, after.Stage(), to)
	case from == to:
		fmt.Printf("oddc: %s %s is the newest\n", after.Stage(), to)
		if after.Original.Rev != "" {
			fmt.Println("Pinned to a commit; --stage main or --stage staging follows a stage.")
		}
		return nil
	default:
		fmt.Printf("oddc: %s %s -> %s\n", after.Stage(), from, to)
	}

	if after.Stage() == "staging" && before.Stage() != "staging" {
		fmt.Println("staging is not released yet; `oddc update --stage main` returns to releases.")
	}

	if !cli.Has(args, "--switch") {
		// A flake that needs untracked files has its own way to rebuild.
		if len(untracked) > 0 {
			return nil
		}

		fmt.Printf(
			"Apply with: sudo nixos-rebuild switch --flake %s\n",
			flake,
		)
		return nil
	}

	return cli.Command(
		"sudo", "nixos-rebuild", "switch", "--flake", flake,
	)
}

// Untracked lists the files of a git flake that git does not track, which
// a git flake leaves out of the build. A flake outside git has none.
func Untracked(flake string) []string {
	out, err := cli.Git(flake, "ls-files", "--others", "--directory", "-z")
	if err != nil {
		return nil
	}

	return cli.NulSeparated(out)
}
