// SPDX-License-Identifier: GPL-3.0-or-later

// Package cli holds what the oddc commands share: flags, paths, where the
// catalog comes from, and running git, gh and other tools.
package cli

import "os"

func Value(
	args []string,
	name string,
	fallback string,
) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}

	return fallback
}

func Values(
	args []string,
	name string,
) []string {
	var result []string

	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			result = append(
				result,
				args[i+1],
			)
		}
	}

	return result
}

func Has(
	args []string,
	name string,
) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}

	return false
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
