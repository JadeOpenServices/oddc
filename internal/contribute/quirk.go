// SPDX-License-Identifier: GPL-3.0-or-later

package contribute

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

const quirkUsage = "usage: oddc quirk enable QUIRK [--device MODEL] [--name KEY] | oddc quirk share"

// RunQuirk adopts a shared quirk for a model, or merges quirk entities that
// hold the same data. Both write to the workspace (or --root) and keep a
// change only when the catalog still validates.
func RunQuirk(args []string) error {
	if len(args) < 2 {
		return errors.New(quirkUsage)
	}
	switch args[1] {
	case "enable":
		return quirkEnable(args)
	case "share":
		return quirkShare(args)
	}
	return errors.New(quirkUsage)
}

// quirkEnable references QUIRK from this machine's model (or --device).
// A quirk may list resolved paths it needs under "requires"; a model
// without one of them cannot enable it. The model then needs evidence
// with quirks.KEY=pass from its own machine before it is verified again.
func quirkEnable(args []string) error {
	if len(args) < 3 || strings.HasPrefix(args[2], "--") {
		return errors.New(quirkUsage)
	}
	quirkID := args[2]

	root, registry, err := loadWorkspace(args)
	if err != nil {
		return err
	}
	quirk, ok := registry.Entities[quirkID]
	if !ok || quirk.Kind != "Quirk" {
		return fmt.Errorf("%s is not a quirk in %s", quirkID, root)
	}

	device := cli.Value(args, "--device", "")
	if device == "" {
		facts, err := cli.ReadFacts(args)
		if err != nil {
			return err
		}
		classification, err := registry.Classify(facts)
		if err != nil {
			return err
		}
		if classification.Result != oddc.ResultMatched {
			return errors.New("no model matches this machine; name it with --device, or draft one with `oddc scaffold`")
		}
		device = classification.Model
	}
	model, ok := registry.Entities[device]
	if !ok || model.Kind != "DeviceModel" {
		return fmt.Errorf("%s is not a device model in %s", device, root)
	}

	quirks, _ := model.Data["quirks"].(map[string]any)
	for key, value := range quirks {
		if entry, _ := value.(map[string]any); entry["ref"] == quirkID {
			return fmt.Errorf("%s already references %s as quirks.%s", device, quirkID, key)
		}
	}

	resolved, err := registry.ResolveModel(device, nil, nil)
	if err != nil {
		return err
	}
	if missing := missingRequirements(quirk.Data, resolved.Resolved); len(missing) > 0 {
		return fmt.Errorf("%s does not fit %s; the model lacks %s", quirkID, device, strings.Join(missing, ", "))
	}

	key := cli.Value(args, "--name", quirkKey(quirkID))
	if _, taken := quirks[key]; taken {
		return fmt.Errorf("%s already has quirks.%s; choose another with --name", device, key)
	}

	path := oddc.EntityPath(root+"/catalog/entities", device)
	if err := rewriteChecked(root, path, func(document map[string]any) {
		data := document["data"].(map[string]any)
		section, _ := data["quirks"].(map[string]any)
		if section == nil {
			section = map[string]any{}
			data["quirks"] = section
		}
		section[key] = map[string]any{"enabled": true, "ref": quirkID}
	}); err != nil {
		return err
	}

	fmt.Printf("%s now enables %s as quirks.%s.\n", device, quirkID, key)
	fmt.Println("Deploy it, test what the quirk does on this machine, then record it:")
	fmt.Printf("  oddc evidence record --root %s --result quirks.%s=pass ...\n", root, key)
	return nil
}

// missingRequirements lists the resolved paths a quirk names under
// "requires" that the model does not have.
func missingRequirements(quirk map[string]any, resolved map[string]any) []string {
	requires, _ := quirk["requires"].(map[string]any)
	var missing []string
	for path := range requires {
		if _, found := oddc.Lookup(resolved, path); !found {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	return missing
}

// quirkKey turns "quirk/common/hybrid-render-on-discrete" into
// "hybridRenderOnDiscrete".
func quirkKey(id string) string {
	name := id[strings.LastIndex(id, "/")+1:]
	var key strings.Builder
	upper := false
	for _, r := range name {
		switch {
		case r == '-':
			upper = true
		case upper:
			key.WriteRune(unicode.ToUpper(r))
			upper = false
		default:
			key.WriteRune(r)
		}
	}
	return key.String()
}

// quirkShare merges quirk entities with the same data into the one with
// the first ID, points every model at it and removes the others. Each
// model that changed needs new evidence. Problems it cannot merge on its
// own are listed with what to do.
func quirkShare(args []string) error {
	root, registry, err := loadWorkspace(args)
	if err != nil {
		return err
	}

	groups := map[string][]string{}
	for id, entity := range registry.Entities {
		if entity.Kind == "Quirk" {
			data, _ := json.Marshal(entity.Data)
			groups[string(data)] = append(groups[string(data)], id)
		}
	}

	replace := map[string]string{}
	for _, ids := range groups {
		sort.Strings(ids)
		for _, id := range ids[1:] {
			replace[id] = ids[0]
		}
	}

	var changed []string
	for _, id := range sortedIDs(registry) {
		entity := registry.Entities[id]
		if entity.Kind != "DeviceModel" {
			continue
		}
		quirks, _ := entity.Data["quirks"].(map[string]any)
		needs := false
		for _, value := range quirks {
			if entry, _ := value.(map[string]any); replace[fmt.Sprint(entry["ref"])] != "" {
				needs = true
			}
		}
		if !needs {
			continue
		}
		path := oddc.EntityPath(root+"/catalog/entities", id)
		if err := rewrite(path, func(document map[string]any) {
			section := document["data"].(map[string]any)["quirks"].(map[string]any)
			for _, value := range section {
				entry := value.(map[string]any)
				if keep := replace[fmt.Sprint(entry["ref"])]; keep != "" {
					entry["ref"] = keep
				}
			}
		}); err != nil {
			return err
		}
		changed = append(changed, id)
	}

	removed := make([]string, 0, len(replace))
	for id := range replace {
		removed = append(removed, id)
	}
	sort.Strings(removed)
	for _, id := range removed {
		if err := os.Remove(oddc.EntityPath(root+"/catalog/entities", id)); err != nil {
			return err
		}
		fmt.Printf("merged %s into %s\n", id, replace[id])
	}

	result := oddc.Validate(root)
	for _, id := range changed {
		fmt.Printf("%s references the shared quirk now; record new evidence for it\n", id)
	}
	if len(removed) == 0 {
		fmt.Println("no quirk entities hold the same data")
	}
	if !result.Valid {
		fmt.Println("still to fix by hand:")
		for _, problem := range result.Errors {
			fmt.Println("  " + problem)
		}
		return errors.New("the catalog still repeats itself")
	}
	return nil
}

func sortedIDs(registry *oddc.Registry) []string {
	ids := make([]string, 0, len(registry.Entities))
	for id := range registry.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// rewrite edits an entity file in place.
func rewrite(path string, edit func(document map[string]any)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	edit(document)
	out, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

// rewriteChecked edits an entity file and puts the old one back when the
// catalog no longer validates.
func rewriteChecked(root, path string, edit func(document map[string]any)) error {
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := rewrite(path, edit); err != nil {
		return err
	}
	if result := oddc.Validate(root); !result.Valid {
		os.WriteFile(path, before, 0o644)
		return fmt.Errorf("not written: %s", strings.Join(result.Errors, "; "))
	}
	return nil
}
