package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/internal/warnings"
	"github.com/coralproject/coral-importer/strategies/csv"
	"github.com/coralproject/coral-importer/strategies/legacy"
	"github.com/coralproject/coral-importer/strategies/legacy/mapper"
	"github.com/coralproject/coral-importer/strategies/livefyre"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
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
	start := time.Now()

	// Configure the writer for the logger. We'll set this in the before hook of
	// the CLI.
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var logFile io.Closer
	defer func() {
		if logFile == nil {
			return
		}

		cancel()
		wg.Wait()
		logFile.Close()
	}()

	app := cli.NewApp()
	app.Name = "github.com/coralproject/coral-importer"
	app.Usage = "imports comment exports from other providers into Coral"
	app.Version = fmt.Sprintf("%v, commit %v, built at %v against migration %d", version, commit, date, CurrentMigrationVersion)
	app.Flags = []cli.Flag{
		&cli.Int64Flag{
			Name:    "migrationID",
			EnvVars: []string{"CORAL_MIGRATION_ID"},
			Usage:   "ID of the most recent migration associated with your installation",
		},
		&cli.StringFlag{
			Name:     "log",
			EnvVars:  []string{"CORAL_LOG"},
			Required: true,
			Usage:    "output directory for where the logs will be written to",
		},
		&cli.BoolFlag{
			Name:  "forceSkipMigrationCheck",
			Usage: "used to skip the migration version check",
		},
		&cli.BoolFlag{
			Name:  "disableMonotonicCursorTimes",
			Usage: "used to disable monotonic cursor times which adds a offset to the same times to ensure all emitted times are unique",
		},
		&cli.DurationFlag{
			Name:  "memoryStatFrequency",
			Usage: "specify the frequency of measurements of memory usage, default is never",
		},
	}
	app.Before = func(c *cli.Context) error {
		// Configure the logger.
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.JSONFormatter{})

		f, err := os.OpenFile(c.String("log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		logFile = f

		logrus.SetOutput(f)

		memoryStatFrequency := c.Duration("memoryStatFrequency")
		if memoryStatFrequency > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				StartLoggingMemoryStats(ctx, memoryStatFrequency)
			}()
		}

		// Check that the imported needs updating.
		if c.Bool("forceSkipMigrationCheck") {
			logrus.Warn("skipping migration check")
		} else if c.Int64("migrationID") != CurrentMigrationVersion {
			logrus.WithFields(logrus.Fields{
				"migrationID":             c.Int("migrationID"),
				"currentMigrationVersion": CurrentMigrationVersion,
			}).Fatal("migration version mismatch, update importer to support new migrations or skip with --forceSkipMigrationCheck")
		}

		// Add support for the monotonic cursor times if not disabled.
		if c.Bool("disableMonotonicCursorTimes") {
			logrus.Warn("monotonic cursor times are disabled, some entries may have duplicate cursor times")
		} else {
			logrus.Info("monotonic cursor times are enabled, cursor times will be offset automatically")
			coral.EnableMonotonicCursorTime()
		}

		color.New(color.Bold).Printf("coral-importer (%s)\n", c.App.Version)

		return nil
	}
	app.Commands = []*cli.Command{
		{
			Name:   "csv",
			Usage:  "a migrator designed to migrate data from the standardized CSV format",
			Action: csv.CLI,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "tenantID",
					Usage:    "ID of the Tenant to import for",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "siteID",
					Usage:    "ID of the Site to import for",
					Required: true,
				},
				&cli.StringFlag{
					Name:  "auth",
					Usage: "type of profile to emit (One of \"sso\" or \"local\")",
					Value: "sso",
				},
				&cli.StringFlag{
					Name:     "input",
					Usage:    "folder where the CSV input files are located",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "output",
					Usage:    "folder where the outputted mongo files should be placed",
					Required: true,
				},
				&cli.BoolFlag{
					Name:  "dryRun",
					Usage: "processes data to validate inputs without actually writing files",
				},
			},
		},
		{
			Name:   "livefyre",
			Usage:  "a migrator designed to migrate data from the LiveFyre platform",
			Action: livefyre.CLI,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "tenantID",
					Usage:    "ID of the Tenant to import for",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "siteID",
					Usage:    "ID of the Site to import for",
					Required: true,
				},
				&cli.BoolFlag{
					Name:  "sso",
					Usage: "when true, enables adding the SSO profile to generated users with the ID of the User",
				},
				&cli.StringFlag{
					Name:     "comments",
					Usage:    "newline separated JSON input file containing comments",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "users",
					Usage:    "newline separated JSON input file containing users",
					Required: true,
				},
				&cli.StringFlag{
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
			Subcommands: []*cli.Command{
				{
					Name:   "map",
					Usage:  "perform mapping of legacy fields into importable files",
					Action: mapper.CLI,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "config",
							EnvVars:  []string{"CORAL_MAPPER_CONFIG"},
							Usage:    "configuration file for the SSO mapping process",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "post",
							EnvVars:  []string{"CORAL_MAPPER_POST_DIRECTORY"},
							Usage:    "directory to write files that have been processed by the mapper",
							Required: true,
						},
					},
				},
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "tenantID",
					EnvVars:  []string{"CORAL_TENANT_ID"},
					Usage:    "ID of the Tenant to import for",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "siteID",
					EnvVars:  []string{"CORAL_SITE_ID"},
					Usage:    "ID of the Site to import for",
					Required: true,
				},
				&cli.StringFlag{
					Name:  "preferredPerspectiveModel",
					Usage: "the preferred model to use for copying over toxicity scores",
					Value: "SEVERE_TOXICITY",
				},
				&cli.StringFlag{
					Name:     "input",
					EnvVars:  []string{"CORAL_INPUT_DIRECTORY"},
					Usage:    "folder where the output from mongoexport is located, separated into collection named JSON files",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "output",
					EnvVars:  []string{"CORAL_OUTPUT_DIRECTORY"},
					Usage:    "folder where the outputted mongo files should be placed",
					Required: true,
				},
				&cli.BoolFlag{
					Name:  "dryRun",
					Usage: "processes data to validate inputs without actually writing files",
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.WithError(err).Fatal()
	}

	warnings.Every(func(warning *warnings.Warning) {
		occurrences := warning.Occurrences()
		if occurrences == 0 {
			return
		}

		logrus.WithFields(logrus.Fields{
			"warning":     warning.String(),
			"occurrences": occurrences,
			"keys":        warning.Keys(),
		}).Warn("warning occurred")
	})

	profiles := warnings.UnsupportedUserProfileProvider.Keys()
	if len(profiles) > 1 {
		logrus.WithFields(logrus.Fields{
			"profiles": profiles,
		}).Warn("multiple forign user profiles found, multiple passes of mapper required")
	} else if len(profiles) == 1 {
		logrus.WithFields(logrus.Fields{
			"profiles": profiles,
		}).Warn("forign user profile found, mapper required")
	}

	color.New(color.Bold, color.FgGreen).Printf("\nCompleted, took %s\n", time.Since(start))
}
