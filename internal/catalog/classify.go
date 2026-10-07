package catalog

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunClassify prints which model the facts match and why, from --facts
// FILE or else the sysfs below --sys. It fails unless exactly one model
// matches.
func RunClassify(registry *oddc.Registry, args []string) error {
	facts, err := cli.ReadFacts(args)
	if err != nil {
		return err
	}

	classification, err := registry.Classify(facts)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(classification, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))

	switch classification.Result {
	case oddc.ResultMatched:
		return nil
	case oddc.ResultAmbiguous:
		return fmt.Errorf(
			"%w: %s",
			oddc.ErrAmbiguousModelMatch,
			strings.Join(classification.Ambiguous, ", "),
		)
	default:
		return oddc.ErrNoModelMatch
	}
}
