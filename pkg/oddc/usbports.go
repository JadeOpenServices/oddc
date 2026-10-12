// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// InternalUSBDevice is a built-in USB device and the port it is wired to:
// its controller (by PCI ID), the root hub (usb2 or usb3), the port number
// and how the port is connected. Consumers trust a device ID as internal
// only on that port; the same ID anywhere else is external. Attachment is
// the component's placement ("internal", or "detachable" for a part such
// as a keyboard on its own connector); Supported is false when the
// component's entity marks it unusable on Linux.
type InternalUSBDevice struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	DeviceID    string `json:"deviceId"`
	Controller  string `json:"controller"`
	Hub         string `json:"hub"`
	Port        int    `json:"port"`
	ConnectType string `json:"connectType"`
	Attachment  string `json:"attachment"`
	Supported   bool   `json:"supported"`
}

// InternalUSBDevices lists the components of a resolved model that have a
// usbPort, sorted by path. A usbPort names its controller by the path of a
// USB controller component (an usbcontroller/ entity) placed in the same model.
func InternalUSBDevices(resolved map[string]any) ([]InternalUSBDevice, error) {
	var found []InternalUSBDevice

	for _, component := range modelComponents(resolved) {
		object := componentAt(resolved, component.Path)
		port, ok := object["usbPort"].(map[string]any)
		if !ok {
			continue
		}

		controllerPath, _ := port["controller"].(string)
		controller := componentAt(resolved, controllerPath)
		controllerID, _ := controller["deviceId"].(string)
		entity, _ := controller["id"].(string)
		if controller == nil || controllerID == "" || !strings.HasPrefix(entity, "usbcontroller/") {
			return nil, fmt.Errorf("%s: usbPort.controller %q is not a USB controller of this model", component.Path, controllerPath)
		}

		hub, _ := port["hub"].(string)
		number, isNumber := portNumber(port["port"])
		connect, _ := port["connectType"].(string)
		switch {
		case hub != "usb2" && hub != "usb3":
			return nil, fmt.Errorf("%s: usbPort.hub %q, want usb2 or usb3", component.Path, hub)
		case !isNumber || number < 1:
			return nil, fmt.Errorf("%s: usbPort.port %v is not a port number", component.Path, port["port"])
		case connect != "hardwired" && connect != "hotplug":
			return nil, fmt.Errorf("%s: usbPort.connectType %q, want hardwired or hotplug", component.Path, connect)
		}

		name, _ := object["name"].(string)
		attachment, _ := object["attachment"].(string)
		status, _ := Lookup(object, "support.linux.status")
		found = append(found, InternalUSBDevice{
			Path: component.Path, Name: name, DeviceID: strings.ToLower(component.DeviceID),
			Controller: controllerID, Hub: hub, Port: number, ConnectType: connect,
			Attachment: attachment, Supported: status != "unsupported",
		})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found, nil
}

// portNumber reads a whole number however the JSON decoder kept it.
func portNumber(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), number == float64(int(number))
	case int:
		return number, true
	case int64:
		return int(number), true
	case json.Number:
		n, err := number.Int64()
		return int(n), err == nil
	}
	return 0, false
}
