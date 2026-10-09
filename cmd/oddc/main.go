// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/catalog"
	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/internal/system"
)

// commands by name, in the order usage lists them: this machine, the
// catalog, then contributing to it.
var commands = []struct {
	name string
	run  func(args []string) error
}{
	{"detect", system.RunDetect},
	{"setup", system.RunSetup},
	{"fetch", system.RunFetch},
	{"doctor", system.RunDoctor},
	{"update", system.RunUpdate},
	{"validate", catalog.Run},
	{"list", catalog.Run},
	{"index", catalog.Run},
	{"classify", catalog.Run},
	{"resolve", catalog.Run},
	{"explain", catalog.Run},
	{"status", catalog.Run},
	{"changes", catalog.RunChanges},
	{"workspace", contribute.RunWorkspace},
	{"scaffold", contribute.RunScaffold},
	{"evidence", contribute.RunEvidence},
	{"contribute", contribute.RunContribute},
}

func run(args []string) error {
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		if len(args) > 0 && args[0] == command.name {
			return command.run(args)
		}
		names = append(names, command.name)
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: oddc <%s>", strings.Join(names, "|"))
	}

	return fmt.Errorf("unknown command %q", args[0])
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(
			os.Stderr,
			"FAIL:",
			err,
		)
		os.Exit(1)
	}
}
