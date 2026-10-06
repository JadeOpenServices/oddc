package oddc

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadIdentity reads a machine's identity from sysfs below sysRoot,
// normally "/sys". Missing values stay empty.
func ReadIdentity(sysRoot string) MachineIdentity {
	dmi := func(name string) string {
		data, err := os.ReadFile(
			filepath.Join(sysRoot, "class", "dmi", "id", name),
		)
		if err != nil {
			return ""
		}

		return strings.TrimSpace(string(data))
	}

	return MachineIdentity{
		FormFactor:     formFactor(sysRoot, dmi("chassis_type")),
		SysVendor:      dmi("sys_vendor"),
		ProductName:    dmi("product_name"),
		ProductVersion: dmi("product_version"),
		BoardVendor:    dmi("board_vendor"),
		BoardName:      dmi("board_name"),
		BoardVersion:   dmi("board_version"),
	}
}

// formFactor prefers a battery as evidence of a laptop, then the SMBIOS
// chassis type.
func formFactor(sysRoot, chassisType string) string {
	batteries, _ := filepath.Glob(
		filepath.Join(sysRoot, "class", "power_supply", "BAT*"),
	)
	if len(batteries) > 0 {
		return "laptop"
	}

	switch chassisType {
	case "8", "9", "10", "11", "14", "30", "31", "32":
		return "laptop"

	case "3", "4", "5", "6", "7", "13", "15", "16", "17", "18", "19",
		"20", "21", "22", "23", "24", "25", "26", "27", "28", "29",
		"33", "34", "35", "36":
		return "desktop"
	}

	return ""
}
