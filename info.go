package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// InfoCmd prints high-level metadata for an extension.
type InfoCmd struct {
	Extension string `arg:"" help:"Extension identifier (publisher.name)."`
}

func (c *InfoCmd) Run(cli *CLI) error {
	pub, name, err := parseExt(c.Extension)
	if err != nil {
		return err
	}
	client := newClient(cli.Source)
	info, err := client.info(pub, name)
	if err != nil {
		return err
	}

	fmt.Printf("Identifier:  %s.%s\n", info.Publisher, info.Name)
	fmt.Printf("Display:     %s\n", info.DisplayName)
	fmt.Printf("Latest:      %s\n", info.Latest)
	fmt.Printf("Versions:    %d total\n", len(info.Versions))
	fmt.Printf("Source:      %s\n", cli.Source)
	if info.Description != "" {
		fmt.Printf("\n%s\n", info.Description)
	}
	return nil
}

// VersionsCmd lists every available version, newest first.
type VersionsCmd struct {
	Extension string `arg:"" help:"Extension identifier (publisher.name)."`
	Limit     int    `short:"n" default:"20" help:"Show only N most recent entries (0 = all)."`
}

func (c *VersionsCmd) Run(cli *CLI) error {
	pub, name, err := parseExt(c.Extension)
	if err != nil {
		return err
	}
	client := newClient(cli.Source)
	versions, err := client.versions(pub, name)
	if err != nil {
		return err
	}

	limit := c.Limit
	if limit <= 0 || limit > len(versions) {
		limit = len(versions)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tPLATFORM\tPRE-RELEASE\tDATE")
	for _, v := range versions[:limit] {
		platform := v.TargetPlatform
		if platform == "" {
			platform = "-"
		}
		pre := "-"
		if v.PreRelease {
			pre = "yes"
		}
		date := v.Date
		if date == "" {
			date = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.Version, platform, pre, date)
	}
	return w.Flush()
}
