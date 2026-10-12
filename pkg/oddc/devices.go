// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import "sort"

// Device is one component of a model as consumers need it: where the model
// places it, what it is, how ODDC matches it, how it is attached, whether it
// is usable on Linux, and what the model's evidence proves about it.
type Device struct {
	// Path is where the model places it, as "hardware.camera.front".
	Path string `json:"path"`
	// ID is the component entity, as "camera/chicony/b5e7".
	ID       string `json:"id"`
	Name     string `json:"name"`
	Bus      string `json:"bus"`
	DeviceID string `json:"deviceId"`
	// Attachment is the model's placement ("internal", "detachable", ...);
	// empty when the model does not say.
	Attachment string `json:"attachment,omitempty"`
	// USBPort is the fixed USB port of a built-in device; nil otherwise.
	USBPort *InternalUSBDevice `json:"usbPort,omitempty"`
	// Supported is false when the component's entity marks it unusable on
	// Linux.
	Supported bool `json:"supported"`
	// Proven is what the model's evidence says of this path: "pass",
	// "unsupported" (present, not usable), or "untested" when the evidence
	// the model's status rests on has no result for it.
	Proven string `json:"proven"`
	// Protocol names the protocol a tool speaks to it, when the component
	// needs one (an Alps touchpad, a button report layout); empty otherwise.
	Protocol string `json:"protocol,omitempty"`
}

// Devices lists every component of a model with a bus and a device ID,
// sorted by path. It is the one inventory consumers and tools read; views
// such as "the built-in USB devices" are filters of it.
func (r *Registry) Devices(model string) ([]Device, error) {
	resolved, err := r.ResolveEntity(model)
	if err != nil {
		return nil, err
	}
	data := resolved.Resolved
	status, err := r.Verify(model)
	if err != nil {
		return nil, err
	}
	results := map[string]any{}
	if status.Evidence != nil {
		results = status.Evidence.Results
	}
	ports := map[string]InternalUSBDevice{}
	internal, err := InternalUSBDevices(data)
	if err != nil {
		return nil, err
	}
	for _, d := range internal {
		ports[d.Path] = d
	}
	unsupported := map[string]bool{}
	for _, part := range UnsupportedComponents(data) {
		unsupported[part.Path] = true
	}

	devices := []Device{}
	for _, c := range modelComponents(data) {
		object := componentAt(data, c.Path)
		d := Device{Path: c.Path, ID: c.ID, Bus: c.Bus, DeviceID: c.DeviceID, Supported: !unsupported[c.Path], Proven: "untested"}
		d.Name, _ = object["name"].(string)
		d.Attachment, _ = object["attachment"].(string)
		d.Protocol = protocolName(object)
		if port, ok := ports[c.Path]; ok {
			d.USBPort = &port
		}
		if result, ok := results[c.Path].(string); ok {
			d.Proven = result
		}
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Path < devices[j].Path })
	return devices, nil
}

// protocolName is the protocol a component or its touchpad names.
func protocolName(object map[string]any) string {
	if name, ok := object["protocol"].(string); ok {
		return name
	}
	if touchpad, ok := object["touchpad"].(map[string]any); ok {
		if name, ok := touchpad["protocol"].(string); ok {
			return name
		}
	}
	return ""
}
