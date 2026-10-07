package oddc

import (
	"os"
	"path/filepath"
	"sort"
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

func stringsAt(root map[string]any, path string) []string {
	value, exists := Lookup(root, path)
	if !exists {
		return nil
	}

	switch typed := value.(type) {
	case string:
		return []string{typed}

	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		result := make([]string, 0, len(keys))

		for _, key := range keys {
			if item, ok := typed[key].(string); ok {
				result = append(result, item)
			}
		}

		return result

	default:
		return nil
	}
}

func stringAt(
	root map[string]any,
	path string,
) (string, bool) {
	value, exists := Lookup(root, path)
	if !exists {
		return "", false
	}

	result, ok := value.(string)
	return result, ok
}

func equalIdentity(a, b string) bool {
	return strings.EqualFold(
		strings.TrimSpace(a),
		strings.TrimSpace(b),
	)
}

// ModelIdentity returns the machine identity a model declares: the first
// value of each DMI field and its class form factor. A machine reporting it
// matches the model.
func (r *Registry) ModelIdentity(id string) (MachineIdentity, error) {
	resolved, err := r.ResolveEntity(id)
	if err != nil {
		return MachineIdentity{}, err
	}

	first := func(path string) string {
		values := stringsAt(resolved.Resolved, path)
		if len(values) == 0 {
			return ""
		}

		return values[0]
	}

	formFactor, _ := stringAt(resolved.Resolved, "class.formFactor")

	return MachineIdentity{
		FormFactor:     formFactor,
		SysVendor:      first("identity.dmi.systemVendor"),
		ProductName:    first("identity.dmi.productName"),
		ProductVersion: first("identity.dmi.productVersion"),
		BoardVendor:    first("identity.dmi.boardVendor"),
		BoardName:      first("identity.dmi.boardName"),
		BoardVersion:   first("identity.dmi.boardVersion"),
	}, nil
}

// SysfsFiles returns the sysfs files, relative to the sysfs root, through
// which a machine reports this identity.
func (identity MachineIdentity) SysfsFiles() map[string]string {
	files := map[string]string{}

	for name, value := range map[string]string{
		"sys_vendor":      identity.SysVendor,
		"product_name":    identity.ProductName,
		"product_version": identity.ProductVersion,
		"board_vendor":    identity.BoardVendor,
		"board_name":      identity.BoardName,
		"board_version":   identity.BoardVersion,
	} {
		if value != "" {
			files[filepath.Join("class", "dmi", "id", name)] = value + "\n"
		}
	}

	switch identity.FormFactor {
	case "laptop":
		files[filepath.Join("class", "power_supply", "BAT0", "type")] = "Battery\n"
	case "desktop":
		files[filepath.Join("class", "dmi", "id", "chassis_type")] = "3\n"
	}

	return files
}
