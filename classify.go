package oddc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ResultMatched   = "matched"
	ResultNone      = "none"
	ResultAmbiguous = "ambiguous"
)

// Facts are what a machine reports about itself: its DMI identity and the
// device IDs ("vvvv:pppp", lower case) it sees per bus. A bus is present
// only when it was read; an empty list means it was read and is empty.
type Facts struct {
	Identity MachineIdentity     `json:"identity"`
	Devices  map[string][]string `json:"devices,omitempty"`
}

// ComponentMatch tells whether a component a model declares is present.
// Present is null when the facts do not cover its bus.
type ComponentMatch struct {
	Path     string `json:"path"`
	ID       string `json:"id"`
	Bus      string `json:"bus"`
	DeviceID string `json:"deviceId"`
	Present  *bool  `json:"present"`
}

// Candidate explains one model against the facts. Only DMI fields choose
// the model; components are reported to explain it.
type Candidate struct {
	Model      string                `json:"model"`
	Name       string                `json:"name"`
	Matched    bool                  `json:"matched"`
	Score      int                   `json:"score"`
	Fields     map[string]FieldMatch `json:"fields"`
	Components []ComponentMatch      `json:"components"`
}

// Classification is the machine-readable answer to "which model is this".
type Classification struct {
	SchemaVersion string      `json:"schemaVersion"`
	Result        string      `json:"result"`
	Model         string      `json:"model,omitempty"`
	Ambiguous     []string    `json:"ambiguous,omitempty"`
	Candidates    []Candidate `json:"candidates"`
}

// Classify picks the model whose declared identity the facts satisfy with
// the most DMI fields. Candidates lists every match and every near miss,
// a model where at least one declared DMI field agrees.
func (r *Registry) Classify(facts Facts) (Classification, error) {
	ids := r.modelIDs()
	sort.Strings(ids)

	result := Classification{
		SchemaVersion: SchemaVersion,
		Result:        ResultNone,
		Candidates:    []Candidate{},
	}

	best := -1
	var winners []string

	for _, id := range ids {
		resolved, err := r.ResolveEntity(id)
		if err != nil {
			return Classification{}, err
		}

		candidate := Candidate{
			Model:   id,
			Name:    resolved.Name,
			Matched: true,
			Fields:  identityFields(resolved.Resolved, facts.Identity),
		}

		for path, field := range candidate.Fields {
			switch {
			case !field.Matched:
				candidate.Matched = false
			case path != "class.formFactor":
				candidate.Score++
			}
		}

		if candidate.Score == 0 {
			continue
		}

		candidate.Components = componentMatches(resolved.Resolved, facts)
		result.Candidates = append(result.Candidates, candidate)

		if !candidate.Matched {
			continue
		}

		switch {
		case candidate.Score > best:
			best = candidate.Score
			winners = []string{id}
		case candidate.Score == best:
			winners = append(winners, id)
		}
	}

	switch len(winners) {
	case 0:
	case 1:
		result.Result = ResultMatched
		result.Model = winners[0]
	default:
		result.Result = ResultAmbiguous
		result.Ambiguous = winners
	}

	return result, nil
}

// factsBus is the bus under which facts list a catalog bus. I2C input
// devices show up as HID devices.
func factsBus(bus string) string {
	if bus == "i2c" {
		return "hid"
	}

	return bus
}

// modelComponents finds every resolved component in a model's data: an
// object carrying an id, a bus and a device ID.
func modelComponents(data map[string]any) []ComponentMatch {
	var found []ComponentMatch

	var walk func(object map[string]any, prefix string)
	walk = func(object map[string]any, prefix string) {
		id, _ := object["id"].(string)
		bus, _ := object["bus"].(string)
		deviceID, _ := object["deviceId"].(string)

		if id != "" && bus != "" && deviceID != "" {
			found = append(found, ComponentMatch{
				Path:     prefix,
				ID:       id,
				Bus:      bus,
				DeviceID: strings.ToLower(deviceID),
			})
		}

		for key, value := range object {
			if child, ok := value.(map[string]any); ok {
				path := key
				if prefix != "" {
					path = prefix + "." + key
				}
				walk(child, path)
			}
		}
	}
	walk(data, "")

	sort.Slice(found, func(i, j int) bool {
		return found[i].Path < found[j].Path
	})

	return found
}

func componentMatches(data map[string]any, facts Facts) []ComponentMatch {
	components := modelComponents(data)

	for i, component := range components {
		seen, read := facts.Devices[factsBus(component.Bus)]
		if !read {
			continue
		}

		present := false
		for _, deviceID := range seen {
			if strings.EqualFold(deviceID, component.DeviceID) {
				present = true
				break
			}
		}
		components[i].Present = &present
	}

	if components == nil {
		return []ComponentMatch{}
	}

	return components
}

// ModelFacts returns facts a machine of this model reports: its declared
// identity and the device IDs of its components.
func (r *Registry) ModelFacts(id string) (Facts, error) {
	identity, err := r.ModelIdentity(id)
	if err != nil {
		return Facts{}, err
	}

	resolved, err := r.ResolveEntity(id)
	if err != nil {
		return Facts{}, err
	}

	devices := map[string][]string{}
	for _, component := range modelComponents(resolved.Resolved) {
		bus := factsBus(component.Bus)
		devices[bus] = append(devices[bus], component.DeviceID)
	}

	return Facts{Identity: identity, Devices: devices}, nil
}

// ReadFacts reads a machine's facts from sysfs below sysRoot.
func ReadFacts(sysRoot string) Facts {
	return Facts{
		Identity: ReadIdentity(sysRoot),
		Devices:  ReadDevices(sysRoot),
	}
}

// ReadDevices lists the PCI, USB and HID device IDs below sysRoot. Buses
// without a sysfs directory are left out.
func ReadDevices(sysRoot string) map[string][]string {
	devices := map[string][]string{}

	read := func(dir, name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return ""
		}

		return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(string(data)), "0x"))
	}

	scan := func(bus string, id func(dir string) string) {
		entries, err := os.ReadDir(filepath.Join(sysRoot, "bus", bus, "devices"))
		if err != nil {
			return
		}

		seen := map[string]bool{}
		list := []string{}

		for _, entry := range entries {
			deviceID := id(filepath.Join(sysRoot, "bus", bus, "devices", entry.Name()))
			if deviceID != "" && !seen[deviceID] {
				seen[deviceID] = true
				list = append(list, deviceID)
			}
		}

		sort.Strings(list)
		devices[bus] = list
	}

	scan("pci", func(dir string) string {
		vendor, device := read(dir, "vendor"), read(dir, "device")
		if vendor == "" || device == "" {
			return ""
		}
		return vendor + ":" + device
	})

	// USB interfaces have no idVendor and are skipped.
	scan("usb", func(dir string) string {
		vendor, product := read(dir, "idVendor"), read(dir, "idProduct")
		if vendor == "" || product == "" {
			return ""
		}
		return vendor + ":" + product
	})

	// HID devices are named BUS:VENDOR:PRODUCT.INSTANCE.
	scan("hid", func(dir string) string {
		name, _, _ := strings.Cut(filepath.Base(dir), ".")
		parts := strings.Split(name, ":")
		if len(parts) != 3 {
			return ""
		}
		return strings.ToLower(parts[1] + ":" + parts[2])
	})

	return devices
}
