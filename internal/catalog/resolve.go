// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import "github.com/JadeOpenServices/oddc/pkg/oddc"

func loadOverlays(
	paths []string,
) ([]oddc.Overlay, error) {
	result := make(
		[]oddc.Overlay,
		0,
		len(paths),
	)

	for _, path := range paths {
		overlay, err := oddc.ReadOverlay(path)
		if err != nil {
			return nil, err
		}

		result = append(
			result,
			overlay,
		)
	}

	return result, nil
}
