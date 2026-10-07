package oddc

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var slugSeparators = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns a DMI value into an ID segment: "Laptop 13 (AMD)" becomes
// "laptop-13-amd".
func slug(value string) string {
	return strings.Trim(slugSeparators.ReplaceAllString(strings.ToLower(value), "-"), "-")
}

// placeholder reports DMI values firmware fills in when a vendor did not
// set a real one.
func placeholder(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default string", "none", "not applicable", "not specified",
		"system product name", "system manufacturer",
		"to be filled by o.e.m.", "type1productconfigid", "0123456789":
		return true
	}

	return false
}

// DraftModel drafts a DeviceModel entity for a machine that no model
// describes yet. The draft holds the machine's DMI system vendor, product
// and board name, its vendor and class when the catalog has them, and
// every catalog component present in facts, placed where other models
// place it. An empty id is derived from the vendor and product.
//
// Notes lists what the draft leaves out: present devices no component
// describes, as "bus vvvv:pppp", and components no model places yet.
func (r *Registry) DraftModel(id string, facts Facts) (Entity, []string, error) {
	identity := facts.Identity
	if placeholder(identity.SysVendor) || placeholder(identity.ProductName) {
		return Entity{}, nil, errors.New("facts lack a DMI system vendor and product name")
	}

	if id == "" {
		id = "model/" + slug(identity.SysVendor) + "/" + slug(identity.ProductName)
	}
	if _, exists := r.Entities[id]; exists {
		return Entity{}, nil, fmt.Errorf("%s already exists", id)
	}

	dmi := map[string]any{}
	for _, field := range []struct{ name, value string }{
		{"systemVendor", identity.SysVendor},
		{"productName", identity.ProductName},
		{"boardName", identity.BoardName},
	} {
		value := strings.TrimSpace(field.value)
		if !placeholder(value) && slug(value) != "" {
			dmi[field.name] = map[string]any{slug(value): value}
		}
	}
	data := map[string]any{"identity": map[string]any{"dmi": dmi}}

	if vendor := "vendor/" + slug(identity.SysVendor); r.Entities[vendor].Kind == "Vendor" {
		data["vendor"] = map[string]any{"ref": vendor}
	}
	if class := "class/" + slug(identity.FormFactor); r.Entities[class].Kind == "DeviceClass" {
		data["class"] = map[string]any{"ref": class}
	}

	present := map[string]bool{}
	for bus, ids := range facts.Devices {
		for _, deviceID := range ids {
			present[bus+" "+strings.ToLower(deviceID)] = true
		}
	}

	placements := r.componentPlacements()
	described := map[string]bool{}
	var notes []string

	for _, componentID := range sortedEntityIDs(r.Entities) {
		component := r.Entities[componentID].Data
		bus, _ := component["bus"].(string)
		deviceID, _ := component["deviceId"].(string)
		device := factsBus(bus) + " " + strings.ToLower(deviceID)
		if bus == "" || deviceID == "" || !present[device] {
			continue
		}
		described[device] = true

		placement, ok := placements[componentID]
		if !ok || !setAt(data, placement.path, cloneObject(placement.value)) {
			notes = append(notes, fmt.Sprintf("%s is %s, which no model places yet", device, componentID))
		}
	}

	for device := range present {
		if !described[device] {
			notes = append(notes, device)
		}
	}
	sort.Strings(notes)

	name := strings.TrimSpace(identity.ProductName)
	if vendor := strings.TrimSpace(identity.SysVendor); !strings.HasPrefix(strings.ToLower(name), strings.ToLower(vendor)) {
		name = vendor + " " + name
	}

	return Entity{
		APIVersion: EntityAPIVersion,
		Kind:       "DeviceModel",
		Metadata:   EntityMetadata{ID: id, Name: name},
		Data:       data,
	}, notes, nil
}

type placement struct {
	path  string
	value map[string]any
}

// componentPlacements finds, for each component some model references,
// the first place a model puts it: the path of the object holding the
// ref and that object, such as {"ref": ..., "attachment": "internal"}.
func (r *Registry) componentPlacements() map[string]placement {
	placements := map[string]placement{}

	var walk func(object map[string]any, path string)
	walk = func(object map[string]any, path string) {
		if ref, ok := object["ref"].(string); ok && path != "" {
			if _, seen := placements[ref]; !seen {
				placements[ref] = placement{path: path, value: object}
			}
		}

		for _, key := range sortedKeys(object) {
			if child, ok := object[key].(map[string]any); ok {
				childPath := key
				if path != "" {
					childPath = path + "." + key
				}
				walk(child, childPath)
			}
		}
	}

	for _, id := range sortedEntityIDs(r.Entities) {
		if entity := r.Entities[id]; entity.Kind == "DeviceModel" {
			walk(entity.Data, "")
		}
	}

	return placements
}

// setAt stores value at a dotted path, creating objects on the way. It
// refuses to replace anything already there.
func setAt(root map[string]any, path string, value any) bool {
	keys := strings.Split(path, ".")
	object := root

	for _, key := range keys[:len(keys)-1] {
		child, exists := object[key]
		if !exists {
			child = map[string]any{}
			object[key] = child
		}

		next, ok := child.(map[string]any)
		if !ok {
			return false
		}
		object = next
	}

	last := keys[len(keys)-1]
	if _, exists := object[last]; exists {
		return false
	}
	object[last] = value

	return true
}

func sortedEntityIDs(entities map[string]Entity) []string {
	ids := make([]string, 0, len(entities))
	for id := range entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids
}
