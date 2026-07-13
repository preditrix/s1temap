// Package cli wires the s1temap command tree with Kong.
package cli

import (
	"fmt"
	"os"

	"preditrix/s1temap/internal/meta"

	"github.com/alecthomas/kong"
)

// CLI is the root Kong grammar for the s1temap command tree.
type CLI struct {
	Start   StartCmd         `cmd:"" help:"Crawl through a sitemap"`
	List    ListCmd          `cmd:"" help:"Crawl through a list of URLs"`
	Tools   ToolsCmd         `cmd:"" help:"Utility tools"`
	Version kong.VersionFlag `help:"Print version and exit"`
}

// Run parses the command line and executes the selected command.
func Run() {
	setupLogging()

	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("s1temap"),
		kong.Description("S1temap - A cmd tool to crawl large SiteMaps from given sources with lots of options."),
		kong.UsageOnError(),
		kong.Vars{"version": meta.VersionString()},
	)

	if err := ctx.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
