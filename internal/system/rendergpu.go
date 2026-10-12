// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunRenderGPU prints the DRM devices for the compositor to open, the one
// to render on first, separated by ':' (aquamarine's AQ_DRM_DEVICES). The
// choice is the model's enabled renderGpu quirk applied to the screens
// connected now (--resolved, default /etc/oddc/resolved.json; --sys, default
// /sys). A model without that quirk gets an error, not a guess. --explain
// writes the reasoning to stderr. Run it when the session starts: a
// compositor keeps its render GPU until it restarts.
func RunRenderGPU(args []string) error {
	data, err := os.ReadFile(cli.Value(args, "--resolved", "/etc/oddc/resolved.json"))
	if err != nil {
		return err
	}
	var resolved map[string]any
	if err := json.Unmarshal(data, &resolved); err != nil {
		return err
	}
	rule, ok, err := oddc.RenderRuleFrom(resolved)
	switch {
	case err != nil:
		return err
	case !ok:
		return errors.New("the resolved model enables no renderGpu quirk; nothing chooses its render GPU")
	}

	sys := cli.Value(args, "--sys", "/sys")
	cards, err := drmCards(sys, oddc.GraphicsDrivers(resolved))
	if err != nil {
		return err
	}
	inUse := map[string]bool{}
	for _, card := range cards {
		if len(card.connected) > 0 {
			inUse[card.gpu] = true
		}
	}
	gpu, err := rule.RenderGPU(inUse)
	if err != nil {
		return err
	}

	// The render GPU first, the others after it in card order.
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].gpu == gpu && cards[j].gpu != gpu })
	var nodes []string
	for _, card := range cards {
		nodes = append(nodes, "/dev/dri/"+card.name)
	}
	if len(nodes) == 0 || cards[0].gpu != gpu {
		return fmt.Errorf("no DRM card for hardware.graphics.%s", gpu)
	}
	if cli.Has(args, "--explain") {
		for _, card := range cards {
			fmt.Fprintf(os.Stderr, "%s: hardware.graphics.%s (%s), connected: %s\n", card.name, card.gpu, card.driver, strings.Join(card.connected, " "))
		}
		fmt.Fprintf(os.Stderr, "render on hardware.graphics.%s (quirks.%s, default %s)\n", gpu, rule.Quirk, rule.Default)
	}
	fmt.Println(strings.Join(nodes, ":"))
	return nil
}

type drmCard struct {
	name, driver, gpu string
	connected         []string
}

// drmCards lists /sys/class/drm/cardN with the GPU key whose driver is
// bound to the card's device, and the card's connected connectors. A card
// whose driver no GPU of the model names is an error.
func drmCards(sys string, drivers map[string]string) ([]drmCard, error) {
	byDriver := map[string]string{}
	for gpu, driver := range drivers {
		byDriver[driver] = gpu
	}
	entries, err := os.ReadDir(filepath.Join(sys, "class", "drm"))
	if err != nil {
		return nil, err
	}
	var cards []drmCard
	index := map[string]int{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		target, err := filepath.EvalSymlinks(filepath.Join(sys, "class", "drm", name, "device", "driver"))
		if err != nil {
			continue
		}
		driver := filepath.Base(target)
		gpu, known := byDriver[driver]
		if !known {
			return nil, fmt.Errorf("%s is driven by %s, which no GPU under hardware.graphics names", name, driver)
		}
		index[name] = len(cards)
		cards = append(cards, drmCard{name: name, driver: driver, gpu: gpu})
	}
	for _, entry := range entries {
		name := entry.Name()
		card, connector, ok := strings.Cut(name, "-")
		if !ok {
			continue
		}
		i, known := index[card]
		if !known {
			continue
		}
		status, err := os.ReadFile(filepath.Join(sys, "class", "drm", name, "status"))
		if err == nil && strings.TrimSpace(string(status)) == "connected" {
			cards[i].connected = append(cards[i].connected, connector)
		}
	}
	return cards, nil
}
