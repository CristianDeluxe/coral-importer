package main

import (
	"fmt"
	"os"

	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/strategies/csv"
	"github.com/coralproject/coral-importer/strategies/legacy"
	"github.com/coralproject/coral-importer/strategies/livefyre"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	// CurrentMigrationVersion is the version representing the most recent migration
	// that this strategy is designed to handle. This should be updated as revisions
	// are applied to this strategy for future versions.
	CurrentMigrationVersion int64 = 1582929716101
)

func main() {
	app := cli.NewApp()
	app.Name = "github.com/coralproject/coral-importer"
	app.Usage = "imports comment exports from other providers into Coral"
	app.Version = fmt.Sprintf("%v, commit %v, built at %v against migration %d", version, commit, date, CurrentMigrationVersion)
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet",
			Usage: "make output quieter",
		},
		cli.BoolFlag{
			Name:  "json",
			Usage: "output logs in JSON",
		},
		cli.Int64Flag{
			Name:  "migrationID",
			Usage: "ID of the most recent migration associated with your installation",
		},
		cli.BoolFlag{
			Name:  "forceSkipMigrationCheck",
			Usage: "used to skip the migration version check",
		},
		cli.BoolFlag{
			Name:  "disableMonotonicCursorTimes",
			Usage: "used to disable monotonic cursor times which adds a offset to the same times to ensure all emitted times are unique",
		},
	}
	app.Before = func(c *cli.Context) error {
		// Configure the logger.
		if err := common.ConfigureLogger(c); err != nil {
			return errors.Wrap(err, "could not configure logger")
		}

		// Check that the imported needs updating.
		if c.GlobalBool("forceSkipMigrationCheck") {
			logrus.Warn("skipping migration check")
		} else if c.GlobalInt64("migrationID") != CurrentMigrationVersion {
			logrus.WithFields(logrus.Fields{
				"migrationID":             c.GlobalInt("migrationID"),
				"currentMigrationVersion": CurrentMigrationVersion,
			}).Fatal("migration version mismatch, update importer to support new migrations or skip with --forceSkipMigrationCheck")
		}

		// Add support for the monotonic cursor times if not disabled.
		if c.GlobalBool("disableMonotonicCursorTimes") {
			logrus.Warn("monotonic cursor times are disabled, some entries may have duplicate cursor times")
		} else {
			logrus.Info("monotonic cursor times are enabled, cursor times will be offset automatically")
			coral.EnableMonotonicCursorTime()
		}

		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:   "csv",
			Usage:  "a migrator designed to migrate data from the standardized CSV format",
			Action: csv.CLI,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "tenantID",
					Usage:    "ID of the Tenant to import for",
					Required: true,
				},
				cli.StringFlag{
					Name:     "siteID",
					Usage:    "ID of the Site to import for",
					Required: true,
				},
				cli.StringFlag{
					Name:  "auth",
					Usage: "type of profile to emit (One of \"sso\" or \"local\")",
					Value: "sso",
				},
				cli.StringFlag{
					Name:     "input",
					Usage:    "folder where the CSV input files are located",
					Required: true,
				},
				cli.StringFlag{
					Name:     "output",
					Usage:    "folder where the outputted mongo files should be placed",
					Required: true,
				},
			},
		},
		{
			Name:   "livefyre",
			Usage:  "a migrator designed to migrate data from the LiveFyre platform",
			Action: livefyre.CLI,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "tenantID",
					Usage:    "ID of the Tenant to import for",
					Required: true,
				},
				cli.StringFlag{
					Name:     "siteID",
					Usage:    "ID of the Site to import for",
					Required: true,
				},
				cli.BoolFlag{
					Name:  "sso",
					Usage: "when true, enables adding the SSO profile to generated users with the ID of the User",
				},
				cli.StringFlag{
					Name:     "comments",
					Usage:    "newline separated JSON input file containing comments",
					Required: true,
				},
				cli.StringFlag{
					Name:     "users",
					Usage:    "newline separated JSON input file containing users",
					Required: true,
				},
				cli.StringFlag{
					Name:     "output",
					Usage:    "folder where the outputted mongo files should be placed",
					Required: true,
				},
			},
		},
		{
			Name:   "legacy",
			Usage:  "a migrator designed to import data from previous versions of Coral",
			Action: legacy.CLI,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "tenantID",
					Usage:    "ID of the Tenant to import for",
					Required: true,
				},
				cli.StringFlag{
					Name:     "siteID",
					Usage:    "ID of the Site to import for",
					Required: true,
				},
				cli.StringFlag{
					Name:  "preferredPerspectiveModel",
					Usage: "the preferred model to use for copying over toxicity scores",
					Value: "SEVERE_TOXICITY",
				},
				cli.StringFlag{
					Name:     "input",
					Usage:    "folder where the output from mongoexport is located, separated into collection named JSON files",
					Required: true,
				},
				cli.StringFlag{
					Name:     "output",
					Usage:    "folder where the outputted mongo files should be placed",
					Required: true,
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.WithError(err).Fatal()
	}
}
