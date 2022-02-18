package csv

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/internal/utility"
	"github.com/coralproject/coral-importer/strategies"
	"github.com/urfave/cli"

	"github.com/microcosm-cc/bluemonday"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

type CommentReference struct {
	ParentID string
}

type StoryReference struct {
	Mode                         string
	CommentStatusCounts          coral.CommentStatusCounts
	CommentModerationQueueCounts coral.CommentModerationQueueCounts
}

type UserReference struct {
	CommentStatusCounts coral.CommentStatusCounts
}

func CLI(c *cli.Context) error {
	return Import(c)
}

// Import will perform the actual import process for the CSV strategy.
func Import(c strategies.Context) error {
	// tenantID is the ID of the Tenant that we are importing these documents
	// for.
	tenantID := c.String("tenantID")

	// siteID is the ID of the Site that we're importing records for.
	siteID := c.String("siteID")

	// output is the name of the folder where we are placing our outputted dumps
	// ready for MongoDB import.
	output := c.String("output")

	// input is the name of the folder where we are loading out collections
	// from the MongoDB export.
	input := c.String("input")

	// auth is the identifier for the type of authentication profiles to be
	// created for the users.
	auth := c.String("auth")
	if auth != "local" && auth != "sso" {
		return errors.Errorf("invalid --auth provided \"%s\", only \"sso\" or \"local\" is supported", auth)
	}

	// Validate that the collection files we expect exist in the input folder.
	if err := validateCollectionFilesExist(input); err != nil {
		return errors.Wrap(err, "could not validate that collection exists")
	}

	commentsInputFileName := filepath.Join(input, "comments.csv")
	storiesInputFileName := filepath.Join(input, "stories.csv")
	usersInputFileName := filepath.Join(input, "users.csv")

	commentsOutputFileName := filepath.Join(output, "comments.json")
	storiesOutputFileName := filepath.Join(output, "stories.json")
	usersOutputFileName := filepath.Join(output, "users.json")

	// Mark when we started.
	started := time.Now()

	startedStoryModeProcessingAt := time.Now()
	logrus.Debug("starting story mode processing")

	stories := make(map[string]StoryReference)

	if err := utility.ReadCSV(storiesInputFileName, StoryColumns, func(line int, fields []string) error {

		s, err := ParseStory(fields)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to parse story")

			return nil
		}

		if s.Mode != "" && s.Mode != "COMMENTS" {
			// Looks like this story has a non-standard story mode! Let's record it.
			stories[s.ID] = StoryReference{
				Mode: s.Mode,
			}
		} else {
			// We don't need to store information about stories that have the default
			// story mode.
			stories[s.ID] = StoryReference{}
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "could not generate story mode map")
	}

	logrus.WithField("took", time.Since(startedStoryModeProcessingAt)).Debug("finished story mode processing")

	logrus.Debug("starting comment map processing")

	comments := make(map[string]CommentReference)
	users := make(map[string]UserReference)

	if err := utility.ReadCSV(commentsInputFileName, CommentColumns, func(line int, fields []string) error {

		c, err := ParseComment(fields)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to parse comment")
			return nil
		}

		// If the story that this comment is on doesn't exist, then skip the
		// comment!
		story, ok := stories[c.StoryID]
		if !ok {
			logrus.WithFields(logrus.Fields{
				"line":      line,
				"commentID": c.ID,
				"storyID":   c.StoryID,
			}).Warn("comment referenced story that doesn't exist")
			return nil
		}

		// Record each comment's status, story ID, and parent ID.

		comments[c.ID] = CommentReference{
			ParentID: c.ParentID,
		}

		// Increment the status counts for the authors comments and the stories
		// comments.

		user := users[c.AuthorID]

		switch c.Status {
		case "APPROVED":
			story.CommentStatusCounts.Approved++
			user.CommentStatusCounts.Approved++
		case "NONE":
			story.CommentModerationQueueCounts.Total++
			story.CommentModerationQueueCounts.Queues.Unmoderated++
			story.CommentStatusCounts.None++
			user.CommentStatusCounts.None++
		case "REJECTED":
			story.CommentStatusCounts.Rejected++
			user.CommentStatusCounts.Rejected++
		}

		stories[c.StoryID] = story
		users[c.AuthorID] = user

		return nil
	}); err != nil {
		return errors.Wrap(err, "could not process comments")
	}

	logrus.WithField("comments", len(comments)).Debug("finished loading comments into map")

	startedReconstructionAt := time.Now()
	logrus.Debug("reconstructing families based on parent id map")

	// Reconstruct all the family relationships from the parentID map.
	reconstructor := common.NewReconstructor()
	for commentID, comment := range comments {
		reconstructor.AddIDs(commentID, comment.ParentID)
	}

	logrus.WithField("took", time.Since(startedReconstructionAt).String()).Debug("finished family reconstruction")

	startedCommentsAt := time.Now()
	logrus.Debug("processing comments")

	commentsWriter, err := utility.NewJSONWriter(commentsOutputFileName)
	if err != nil {
		return errors.Wrap(err, "could not create comment writer")
	}

	// Do this once for each unique policy, and use the policy for the life of the program
	// Policy creation/editing is not safe to use in multiple goroutines
	p := bluemonday.UGCPolicy()

	if err := utility.ReadCSV(commentsInputFileName, CommentColumns, func(line int, fields []string) error {
		c, err := ParseComment(fields)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to parse comment")

			return nil
		}

		createdAt, err := time.Parse(time.RFC3339, c.CreatedAt)
		if err != nil {
			return errors.Wrap(err, "could not parse created_at")
		}
		comment := coral.NewComment(tenantID, siteID)
		comment.ID = c.ID
		comment.AuthorID = c.AuthorID
		comment.StoryID = c.StoryID
		comment.CreatedAt.Time = createdAt

		// Let's handle some story mode specific operations.
		if stories[c.StoryID].Mode == "RATINGS_AND_REVIEWS" {
			if c.ParentID == "" {
				// Rating
				if c.Rating > 0 {
					comment.Rating = c.Rating

					// If the comment has a rating and a body, then it is a review!
					if c.Body != "" {
						comment.Tags = append(comment.Tags, coral.CommentTag{
							Type:      "REVIEW",
							CreatedAt: comment.CreatedAt,
						})
					}
				} else {
					// This comment is a top level comment (no parent id) and does not
					// have a rating, therefore we should add the question tag.
					comment.Tags = append(comment.Tags, coral.CommentTag{
						Type:      "QUESTION",
						CreatedAt: comment.CreatedAt,
					})
				}
			}
		}

		revision := coral.Revision{
			ID:           comment.ID,
			Body:         coral.HTML(p.Sanitize(strings.ReplaceAll(c.Body, "\n", "<br/>"))),
			Metadata:     coral.RevisionMetadata{},
			ActionCounts: map[string]int{},
			CreatedAt: coral.Time{
				Time: createdAt,
			},
		}
		comment.Revisions = []coral.Revision{
			revision,
		}
		comment.ParentID = c.ParentID
		comment.Status = c.Status

		// ID of the parent is the same as the revision ID.
		comment.ParentRevisionID = comment.ParentID

		// Add reconstructed data.
		comment.ChildIDs = reconstructor.GetChildren(comment.ID)
		comment.ChildCount = len(comment.ChildIDs)
		comment.AncestorIDs = reconstructor.GetAncestors(comment.ID)

		if err := common.Check(comment); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to process compiled comment")

			return nil
		}

		if err := commentsWriter.Write(comment); err != nil {
			return errors.Wrap(err, "couldn't write out comment")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "could not process comments")
	}

	if err := commentsWriter.Close(); err != nil {
		return errors.Wrap(err, "could not finish writing out comments")
	}

	logrus.WithFields(logrus.Fields{
		"took": time.Since(startedCommentsAt).String(),
	}).Debug("finished processing comments")

	startedUsersAt := time.Now()
	logrus.Debug("processing users")

	usersWriter, err := utility.NewJSONWriter(usersOutputFileName)
	if err != nil {
		return errors.Wrap(err, "could not create users writer")
	}

	if err := utility.ReadCSV(usersInputFileName, UserColumns, func(line int, fields []string) error {
		u, err := ParseUser(fields)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to parse user")

			return nil
		}

		// Parse the user from the file.
		user := coral.NewUser(tenantID)
		user.ID = u.ID
		user.Email = u.Email

		// Get the status counts for this user.
		user.CommentCounts.Status = users[user.ID].CommentStatusCounts

		// created_at
		if u.CreatedAt != "" {
			createdAt, err := time.Parse(time.RFC3339, u.CreatedAt)
			if err != nil {
				return errors.Wrap(err, "could not parse created_at")
			}

			user.CreatedAt.Time = createdAt
		}

		// username
		user.Username = u.Username
		user.Status.UsernameStatus.History = []coral.UserUsernameStatusHistory{
			{
				ID:        uuid.NewV1().String(),
				Username:  user.Username,
				CreatedBy: user.ID,
				CreatedAt: user.CreatedAt,
			},
		}

		// role
		user.Role = u.Role

		// banned
		if u.Banned != "" {
			banned, err := strconv.ParseBool(u.Banned)
			if err != nil {
				return errors.Wrap(err, "could not parse banned")
			}

			if banned {
				user.Status.BanStatus.Active = true
				user.Status.BanStatus.History = []coral.UserBanStatusHistory{
					{
						ID:        uuid.NewV1().String(),
						Message:   "",
						Active:    true,
						CreatedAt: user.CreatedAt,
					},
				}
			}
		}

		switch auth {
		case "local":
			user.Profiles = []coral.UserProfile{
				{
					ID:         user.Email,
					Type:       "local",
					Password:   uuid.NewV4().String(),
					PasswordID: uuid.NewV4().String(),
				},
			}
		case "sso":
			user.Profiles = []coral.UserProfile{
				{
					ID:           user.ID,
					Type:         "sso",
					LastIssuedAt: &user.CreatedAt,
				},
			}
		}

		// Check the user.
		if err := common.Check(user); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to process compiled user")

			return nil
		}

		if err := usersWriter.Write(user); err != nil {
			return errors.Wrap(err, "couldn't write out user")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "could not process users")
	}

	if err := usersWriter.Close(); err != nil {
		return errors.Wrap(err, "could not finish writing out users")
	}

	logrus.WithFields(logrus.Fields{
		"took": time.Since(startedUsersAt).String(),
	}).Debug("finished processing users")

	startedStoriesAt := time.Now()
	logrus.Debug("processing stories")

	storiesWriter, err := utility.NewJSONWriter(storiesOutputFileName)
	if err != nil {
		return errors.Wrap(err, "could not create story writer")
	}

	if err := utility.ReadCSV(storiesInputFileName, StoryColumns, func(line int, fields []string) error {
		s, err := ParseStory(fields)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to parse story")

			return nil
		}

		story := coral.NewStory(tenantID, siteID)
		story.ID = s.ID
		story.URL = s.URL

		// Get the status counts for this story.
		story.CommentCounts.Status = stories[story.ID].CommentStatusCounts
		story.CommentCounts.ModerationQueue = stories[story.ID].CommentModerationQueueCounts

		// Copy over the metadata.
		story.Metadata.Title = s.Title
		story.Metadata.Author = s.Author

		if s.PublishedAt != "" {
			publishedAt, err := time.Parse(time.RFC3339, s.PublishedAt)
			if err != nil {
				return errors.Wrap(err, "could not parse published_at")
			}

			story.Metadata.PublishedAt = &coral.Time{
				Time: publishedAt,
			}
		}

		// Copy over the closed at date if provided.
		if s.ClosedAt != "" {
			closedAt, err := time.Parse(time.RFC3339, s.ClosedAt)
			if err != nil {
				return errors.Wrap(err, "could not parse closed_at")
			}

			story.ClosedAt = &coral.Time{
				Time: closedAt,
			}
		}

		// Add the story mode if not default.
		if s.Mode != "" && s.Mode != "COMMENTS" {
			story.Settings.Mode = &s.Mode
		}

		if err := common.Check(story); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"line":   line,
				"fields": fields,
			}).Warn("failed to process compiled story")

			return nil
		}

		if err := storiesWriter.Write(story); err != nil {
			return errors.Wrap(err, "couldn't write out story")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "could not process stories")
	}

	if err := storiesWriter.Close(); err != nil {
		return errors.Wrap(err, "could not finish writing out comments")
	}

	logrus.WithFields(logrus.Fields{
		"took": time.Since(startedStoriesAt).String(),
	}).Debug("finished processing stories")

	// Mark when we finished.
	logrus.WithField("took", time.Since(started).String()).Info("finished processing")

	return nil
}

// validateCollectionFilesExist will ensure that all the collection files that
// we expect to be in the input directory actually exist.
func validateCollectionFilesExist(input string) error {
	collections := []string{
		"users",
		"stories",
		"comments",
	}

	for _, collection := range collections {
		filePath := filepath.Join(input, fmt.Sprintf("%s.csv", collection))
		if !utility.Exists(filePath) {
			return errors.Errorf("%s does not exist", filePath)
		}
	}

	return nil
}
