package mapper

import (
	"path/filepath"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// CLI is the actual task ran when running this tool.
func CLI(c *cli.Context) error {
	// input is the name of the folder where we are loading out collections
	// from the MongoDB export.
	input := c.String("input")

	// output is the name of the folder where there is the files that have already
	// been processed by the importer.
	output := c.String("output")

	// post is the name of the folder where we are placing our outputted dumps
	// ready for MongoDB import.
	post := c.String("post")

	// dryRun indicates that the strategy should not write files and is used for
	// validation.
	dryRun := c.Bool("dryRun")

	if dryRun {
		color.New(color.Bold, color.FgRed).Println("--dryRun is enabled, files will not be written")
		logrus.Warn("dry run is enabled, files will not be written")
	}

	m := New(dryRun)

	// Load the configuration and compile the replacement expressions.
	if err := m.LoadConfig(); err != nil {
		return errors.Wrap(err, "could not load the config")
	}

	// Load all the updates for users in the --pre file.
	if err := m.Pre(filepath.Join(input, "users.json")); err != nil {
		return errors.Wrap(err, "could not load the pre users")
	}

	// Process all the updates to the post file.
	if err := m.Post(filepath.Join(output, "users.json"), filepath.Join(post, "users.json")); err != nil {
		return errors.Wrap(err, "could not load the post users")
	}

	return nil
}
