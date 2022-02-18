package livefyre

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/coralproject/coral-importer/common/pipeline"
	"github.com/coralproject/coral-importer/strategies"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Time is the time.Time representation that LiveFyre uses.
type Time struct {
	time.Time
}

// UnmarshalJSON is the custom unmarshaler for the input JSON from LiveFyre.
func (t *Time) UnmarshalJSON(buf []byte) error {
	tt, err := time.Parse("2006-01-02T15:04:05", strings.Trim(string(buf), `"`))
	if err != nil {
		return errors.Wrap(err, "could not parse livefyre time")
	}

	t.Time = tt

	return nil
}

func CLI(c *cli.Context) error {
	return Import(c)
}

// Import will handle a data import task for importing comments into Coral
// from a LiveFyre export.
func Import(c strategies.Context) error {
	// tenantID is the ID of the Tenant that we are importing these documents
	// for.
	tenantID := c.String("tenantID")

	// siteID is the ID of the Site that we're importing records for.
	siteID := c.String("siteID")

	// commentsFileName is the name of the file that that contains the comment
	// exports from LiveFyre.
	commentsFileName := c.String("comments")

	// usersFileName is the name of the file that that contains the comment
	// exports from LiveFyre.
	usersFileName := c.String("users")

	// folder is the name of the folder where we are placing our outputted dumps
	// ready for MongoDB import.
	folder := c.String("output")

	// sso when true indicates that we should create a sso profile for generated
	// users.
	sso := c.Bool("sso")

	// Mark when we started.
	started := time.Now()
	logrus.Info("started")

	// Process the users file first because we need to de-duplicate users as
	// they are parsed because LiveFyre did not lowercase email addresses,
	// causing multiple users to be created for each email address variation.
	users, err := pipeline.NewMapAggregator(
		pipeline.MergeTaskAggregatorOutputPipelines(
			pipeline.FanAggregatingProcessor(
				pipeline.NewJSONFileReader(usersFileName),
				ProcessUsersMap(),
			),
		),
	)
	if err != nil {
		return errors.Wrap(err, "could not aggregate users")
	}

	logrus.WithField("users", len(users["id"])).Info("loaded users")

	// Genreate the users association from the id map.
	uniqueUsers := make(map[string]string)
	for _, ids := range users["id"] {
		for _, id := range ids {
			uniqueUsers[id] = ids[0]
		}
	}

	// Create the processor that will write these entries out.
	if err := pipeline.NewFileWriter(
		folder,
		pipeline.MergeTaskWriterOutputPipelines(
			pipeline.FanWritingProcessors(
				pipeline.NewJSONFileReader(commentsFileName),
				ProcessComments(tenantID, siteID, uniqueUsers),
			),
		),
	); err != nil {
		return errors.Wrap(err, "could not process comments and stories for writing")
	}

	// Load all the comment statuses by reading the comments.json file again.
	statusCounts, err := pipeline.NewSummer(
		pipeline.MergeTaskSummerOutputPipelines(
			pipeline.FanSummerProcessor(
				pipeline.NewJSONFileReader(filepath.Join(folder, "comments.json")),
				ProcessCommentStatusMap(),
			),
		),
	)
	if err != nil {
		return errors.Wrap(err, "could not process status counts")
	}

	if err := pipeline.NewFileWriter(
		folder,
		ProcessUsers(tenantID, sso, users, statusCounts),
	); err != nil {
		return errors.Wrap(err, "could not write out users")
	}

	logrus.WithField("users", len(users["id"])).Info("wrote users")

	// Mark when we finished.
	finished := time.Now()
	logrus.WithField("took", finished.Sub(started).String()).Info("finished processing")

	return nil
}
