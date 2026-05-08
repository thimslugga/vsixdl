package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

// CLI is the root grammar.
type CLI struct {
	Download DownloadCmd `cmd:"" aliases:"dl,get" help:"Download a VS Code extension as a .vsix file."`
	Info     InfoCmd     `cmd:""                  help:"Show extension metadata."`
	Versions VersionsCmd `cmd:""                  help:"List available versions for an extension."`

	Source string `short:"s" enum:"ms-marketplace,openvsx" default:"ms-marketplace" help:"Registry to query (ms-marketplace|openvsx)." env:"VSIXDL_SOURCE"`
	Quiet  bool   `short:"q" help:"Suppress progress and informational output." env:"VSIXDL_QUIET"`

	Version kong.VersionFlag `short:"V" name:"version" help:"Print version and quit."`
}

func main() {
	// Show help when invoked with no arguments instead of an error.
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "--help")
	}

	cli := CLI{}
	ctx := kong.Parse(&cli,
		kong.Name("vsixdl"),
		kong.Description("Download VS Code extensions (.vsix) from the MS Marketplace or OpenVSX."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Vars{"version": version},
	)

	if err := ctx.Run(&cli); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
