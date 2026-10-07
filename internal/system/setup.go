package system

import (
	"fmt"

	"github.com/JadeOpenServices/oddc/internal/cli"
)

func RunSetup(args []string) error {
	registry, model, err := Detect(args)
	if err != nil {
		return err
	}

	if deployed := cli.DeployedModel(cli.SystemRoot); deployed == model {
		fmt.Printf("Already set up: %s\n", model)
		return nil
	}

	fmt.Printf(`# This machine: %s

# flake.nix
inputs.oddc.url = "%s";

# NixOS configuration (pass inputs to your modules)
{
  imports = [ inputs.oddc.nixosModules.default ];
  oddc.device = "%s";
}
`,
		registry.Entities[model].Metadata.Name,
		cli.Upstream,
		model,
	)

	return nil
}
