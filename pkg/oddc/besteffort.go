// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReportEndpoint is where the owner of a machine no model matches can send
// what `oddc devices` found, once openjade.de runs a server for it. It is
// empty until then: nothing is sent anywhere, and consumers point to the
// contributing guide instead. It changes only through a reviewed change to
// this file, never through configuration on the machine, and has to be an
// https:// URL.
const ReportEndpoint = ""

// ContributingGuide tells people how to add their machine to the catalog.
const ContributingGuide = "https://github.com/JadeOpenServices/oddc/blob/stable/CONTRIBUTING.md"

// Contribute says how the owner of a machine no model matches can help
// make it a proven model.
type Contribute struct {
	// ReportEndpoint is ReportEndpoint, or null while it is empty.
	ReportEndpoint *string `json:"reportEndpoint"`
	Guide          string  `json:"guide"`
	// Commands add the machine to the catalog, in order.
	Commands []string `json:"commands"`
}

// ContributeInfo is the Contribute of this ODDC revision.
func ContributeInfo() Contribute {
	info := Contribute{
		Guide:    ContributingGuide,
		Commands: []string{"oddc scaffold", "oddc evidence", "oddc contribute"},
	}
	if ReportEndpoint != "" {
		endpoint := ReportEndpoint
		info.ReportEndpoint = &endpoint
	}
	return info
}

// PresentPart is a device of a machine no model matches, with the catalog
// component that has its ID, if any. Nothing about it is proven: a
// component's own facts prove nothing for a machine nobody tested.
type PresentPart struct {
	Bus      string `json:"bus"`
	DeviceID string `json:"deviceId"`
	// ID is the catalog component with this bus and device ID; empty when
	// the catalog has none.
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// Supported is false when the component's entity marks it unusable on
	// Linux, with the reason in UnsupportedReason.
	Supported         bool   `json:"supported"`
	UnsupportedReason string `json:"unsupportedReason,omitempty"`
	// Drivers are the kernel drivers bound to it now.
	Drivers []string `json:"drivers,omitempty"`
	Proven  string   `json:"proven"`
}

// PresentParts lists every device the facts report, each with its catalog
// component when one has its bus and device ID.
func (r *Registry) PresentParts(facts Facts, bound map[string]map[string][]string) []PresentPart {
	components := map[string]string{}
	for _, id := range sortedEntityIDs(r.Entities) {
		data := r.Entities[id].Data
		bus, _ := data["bus"].(string)
		deviceID, _ := data["deviceId"].(string)
		key := factsBus(bus) + " " + strings.ToLower(deviceID)
		if bus != "" && deviceID != "" && components[key] == "" {
			components[key] = id
		}
	}

	parts := []PresentPart{}
	for bus, ids := range facts.Devices {
		for _, deviceID := range ids {
			deviceID = strings.ToLower(deviceID)
			part := PresentPart{Bus: bus, DeviceID: deviceID, Supported: true, Proven: "untested", Drivers: bound[bus][deviceID]}
			if id := components[bus+" "+deviceID]; id != "" {
				data := r.Entities[id].Data
				part.ID = id
				part.Name = r.Entities[id].Metadata.Name
				if status, _ := Lookup(data, "support.linux.status"); status == "unsupported" {
					part.Supported = false
					reason, _ := Lookup(data, "support.linux.reason")
					part.UnsupportedReason, _ = reason.(string)
				}
			}
			parts = append(parts, part)
		}
	}
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].Bus != parts[j].Bus {
			return parts[i].Bus < parts[j].Bus
		}
		return parts[i].DeviceID < parts[j].DeviceID
	})
	return parts
}

// GPU is a PCI display controller (class 0x03) of this machine.
type GPU struct {
	// Slot is its PCI address, as "0000:01:00.0".
	Slot     string `json:"slot"`
	DeviceID string `json:"deviceId"`
	// ID is the catalog component with this device ID; empty when none.
	ID string `json:"id"`
	// Driver is the kernel driver bound to it now; empty when none.
	Driver string `json:"driver,omitempty"`
	// Mode is "native" when its driver drives a DRM card of its own, and
	// "basic" otherwise: the screen then runs on the firmware framebuffer
	// (simpledrm, efifb), or not at all.
	Mode string `json:"mode"`
	// Removable is true when the kernel marks it as behind an external
	// port (Thunderbolt, USB4). Such a GPU gets no options: anyone can plug
	// one in, with any ID.
	Removable bool `json:"removable"`
	// Options are the driver choices proven for this exact chip on other
	// models. None of them is proven on this machine.
	Options []GraphicsOption `json:"options"`
}

// GraphicsOption is a driver branch a verified model uses for a GPU with
// the same PCI vendor and device ID. It comes from that model's policy and
// evidence, so the model stays the only owner of it.
type GraphicsOption struct {
	// DriverBranch is the model's policy.graphics.<role>.driverBranch.
	DriverBranch string `json:"driverBranch"`
	// Drivers were bound to the GPU in the evidence the model is verified
	// by; the boot check expects the first one.
	Drivers []string `json:"drivers"`
	// ProvenOn is the model, with what the evidence ran on.
	ProvenOn ProvenOn `json:"provenOn"`
}

// ProvenOn is a verified model and the evidence a GraphicsOption rests on.
type ProvenOn struct {
	Model    string `json:"model"`
	Name     string `json:"name"`
	Evidence string `json:"evidence"`
	Kernel   string `json:"kernel,omitempty"`
	OS       string `json:"os,omitempty"`
}

// ReadGPUs lists the PCI display controllers under sysRoot, with the
// driver bound to each and whether that driver made a DRM card for it.
func ReadGPUs(sysRoot string) []GPU {
	dir := filepath.Join(sysRoot, "bus", "pci", "devices")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	gpus := []GPU{}
	for _, entry := range entries {
		device := filepath.Join(dir, entry.Name())
		if !strings.HasPrefix(readID(device, "class"), "03") {
			continue
		}
		id := busIDs["pci"](device)
		if id == "" {
			continue
		}
		gpu := GPU{Slot: entry.Name(), DeviceID: id, Mode: "basic", Options: []GraphicsOption{}}
		if link, err := os.Readlink(filepath.Join(device, "driver")); err == nil {
			gpu.Driver = filepath.Base(link)
		}
		if cards, _ := filepath.Glob(filepath.Join(device, "drm", "card*")); gpu.Driver != "" && len(cards) > 0 {
			gpu.Mode = "native"
		}
		removable, _ := os.ReadFile(filepath.Join(device, "removable"))
		gpu.Removable = strings.TrimSpace(string(removable)) == "removable"
		gpus = append(gpus, gpu)
	}
	sort.Slice(gpus, func(i, j int) bool { return gpus[i].Slot < gpus[j].Slot })
	return gpus
}

// GraphicsOptions fills in each GPU's catalog component and its options:
// for every verified model that places a component with the GPU's exact
// device ID at hardware.graphics.<role> and sets
// policy.graphics.<role>.driverBranch, with a driver bound to it in the
// evidence it is verified by. No family or class of GPU matches, only the
// same chip. A removable GPU gets none.
func (r *Registry) GraphicsOptions(gpus []GPU) ([]GPU, error) {
	type proven struct {
		component string
		option    GraphicsOption
	}
	byDevice := map[string][]proven{}

	for _, model := range sortedEntityIDs(r.Entities) {
		if r.Entities[model].Kind != "DeviceModel" {
			continue
		}
		resolved, err := r.ResolveEntity(model)
		if err != nil {
			return nil, err
		}
		var candidates []ComponentMatch
		for _, c := range modelComponents(resolved.Resolved) {
			role, ok := strings.CutPrefix(c.Path, "hardware.graphics.")
			if !ok || c.Bus != "pci" || strings.Contains(role, ".") {
				continue
			}
			if branch, _ := Lookup(resolved.Resolved, "policy.graphics."+role+".driverBranch"); branch != nil {
				candidates = append(candidates, c)
			}
		}
		if len(candidates) == 0 {
			continue
		}

		status, err := r.Verify(model)
		if err != nil {
			return nil, err
		}
		if status.Status != Verified || status.Evidence == nil {
			continue
		}
		for _, c := range candidates {
			drivers := status.Evidence.Drivers[c.ID]
			if len(drivers) == 0 {
				continue
			}
			role := strings.TrimPrefix(c.Path, "hardware.graphics.")
			branch, _ := Lookup(resolved.Resolved, "policy.graphics."+role+".driverBranch")
			text, _ := branch.(string)
			if text == "" {
				continue
			}
			kernel, _ := status.Evidence.Environment["kernel"].(string)
			system, _ := status.Evidence.Environment["os"].(string)
			byDevice[c.DeviceID] = append(byDevice[c.DeviceID], proven{c.ID, GraphicsOption{
				DriverBranch: text,
				Drivers:      drivers,
				ProvenOn: ProvenOn{
					Model:    model,
					Name:     r.Entities[model].Metadata.Name,
					Evidence: status.Evidence.ID,
					Kernel:   kernel,
					OS:       system,
				},
			}})
		}
	}

	for i := range gpus {
		for _, p := range byDevice[gpus[i].DeviceID] {
			gpus[i].ID = p.component
			if !gpus[i].Removable {
				gpus[i].Options = append(gpus[i].Options, p.option)
			}
		}
		if gpus[i].ID == "" {
			gpus[i].ID = r.componentWithID("pci", gpus[i].DeviceID)
		}
	}
	return gpus, nil
}

// componentWithID is the first catalog component on bus with deviceID, or
// "".
func (r *Registry) componentWithID(bus, deviceID string) string {
	for _, id := range sortedEntityIDs(r.Entities) {
		data := r.Entities[id].Data
		b, _ := data["bus"].(string)
		d, _ := data["deviceId"].(string)
		if b == bus && strings.EqualFold(d, deviceID) {
			return id
		}
	}
	return ""
}

// LiveBoot is how the installer's live system booted on this machine: the
// one boot of this exact unit that is known to work.
type LiveBoot struct {
	// Parameters are the graphics-safety parameters on its kernel command
	// line ("nomodeset", "DRIVER.modeset=0"). When the live system needed
	// them, the installed base needs them too, until a graphics option
	// passes its trial.
	Parameters []string `json:"parameters"`
}

// SafeGraphicsParameters picks the graphics-safety parameters out of a
// kernel command line and nothing else.
func SafeGraphicsParameters(cmdline string) []string {
	found := []string{}
	for _, word := range strings.Fields(cmdline) {
		driver, isModeset := strings.CutSuffix(word, ".modeset=0")
		if word == "nomodeset" || (isModeset && driverName.MatchString(driver)) {
			found = append(found, word)
		}
	}
	return found
}
