package legacy

import (
	"os"
	"runtime"
	"time"

	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/internal/utility"
	"github.com/coralproject/coral-importer/strategies"
	easyjson "github.com/mailru/easyjson"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// PreferredPerspectiveModel is the stored preferred perspective model that
// should be used to copy over the perspective settings.
var PreferredPerspectiveModel string

func validateExists(filenames ...string) error {
	for _, filename := range filenames {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return errors.Errorf("%s does not exist", filename)
		}
	}

	return nil
}

func CLI(c *cli.Context) error {
	return Import(c)
}

// Import will handle a data import task for importing comments into Coral from
// a legacy export.
func Import(c strategies.Context) error {
	// Copy over the preferredPerspectiveModel from the flags.
	PreferredPerspectiveModel = c.String("preferredPerspectiveModel")

	// Create the new context to operate with.
	ctx := NewContext(c)

	if err := validateExists(
		ctx.Filenames.Input.Comments,
		ctx.Filenames.Input.Actions,
		ctx.Filenames.Input.Assets,
		ctx.Filenames.Input.Users,
	); err != nil {
		return errors.Wrap(err, "required file/folder missing")
	}

	// Mark when we started.
	started := time.Now()

	if err := SeedCommentsReferences(ctx); err != nil {
		return errors.Wrap(err, "could not process comment map")
	}

	logrus.WithField("comments", len(ctx.comments)).Debug("finished loading comments into map")

	if err := ProcessCommentActions(ctx); err != nil {
		return errors.Wrap(err, "could not process comment actions")
	}

	logrus.Debug("finished writing out comment actions")

	startedReconstructionAt := time.Now()
	logrus.Debug("reconstructing families based on parent id map")

	// Reconstruct all the family relationships from the parentID map.
	for commentID, comment := range ctx.comments {
		ctx.Reconstructor.AddIDs(commentID, comment.ParentID)
	}

	logrus.WithField("took", time.Since(startedReconstructionAt).String()).Debug("finished family reconstruction")

	// Load all the comments in from the comments.json file.
	if err := ProcessComments(ctx); err != nil {
		return errors.Wrap(err, "could not read comments json")
	}

	// Release the comments then garbage collect.
	ctx.ReleaseComments()
	runtime.GC()

	// Process the stories now.
	if err := ProcessStories(ctx); err != nil {
		return errors.Wrap(err, "could not process stories")
	}

	// Release the stories then garbage collect.
	ctx.ReleaseStories()
	runtime.GC()

	if err := ProcessUsers(ctx); err != nil {
		return errors.Wrap(err, "could not process users")
	}

	// Mark when we finished.
	finished := time.Now()
	logrus.WithField("took", finished.Sub(started).String()).Info("finished processing")

	return nil
}

func SeedCommentsReferences(ctx *Context) error {
	return utility.ReadJSON(ctx.Filenames.Input.Comments, func(line int, data []byte) error {
		var in Comment
		if err := easyjson.Unmarshal(data, &in); err != nil {
			logrus.WithField("line", line).Error(err)

			return errors.Wrap(err, "could not parse a comment")
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			logrus.WithError(err).WithField("line", line).Warn("validation failed for input user")

			return nil
		}

		ref, _ := ctx.FindOrCreateComment(in.ID)
		ref.Status = TranslateCommentStatus(in.Status)
		ref.StoryID = in.AssetID
		if in.ParentID != nil {
			ref.ParentID = *in.ParentID
		}

		return nil
	})
}

func ProcessCommentActions(ctx *Context) error {
	commentActionsWriter, err := utility.NewJSONWriter(ctx.Filenames.Output.CommentActions)
	if err != nil {
		return errors.Wrap(err, "could not create commentActionsWriter")
	}
	defer commentActionsWriter.Close()

	return utility.ReadJSON(ctx.Filenames.Input.Actions, func(line int, data []byte) error {
		// Parse the Action from the file.
		var in Action
		if err := easyjson.Unmarshal(data, &in); err != nil {
			logrus.WithField("line", line).Error(err)

			return errors.Wrap(err, "could not parse an action")
		}

		// Ignore the action if it's not a comment action.
		if in.ItemType != "COMMENTS" {
			logrus.WithField("line", line).Warn("skipping non-comment action")

			return nil
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			return errors.Wrap(err, "checking failed input Action")
		}

		// Translate the action to a comment action.
		action := TranslateCommentAction(ctx.TenantID, ctx.SiteID, &in)

		// Find the comment's reference.
		ref, ok := ctx.FindComment(action.CommentID)
		if !ok {
			return nil
		}

		action.StoryID = ref.StoryID

		story, _ := ctx.FindOrCreateStory(ref.StoryID)

		ref.ActionCounts[action.ActionType]++
		story.ActionCounts[action.ActionType]++
		if action.ActionType == "FLAG" {
			ref.ActionCounts[action.ActionType+"__"+action.Reason]++
			story.ActionCounts[action.ActionType+"__"+action.Reason]++
		}

		if err := commentActionsWriter.Write(action); err != nil {
			return errors.Wrap(err, "couldn't write out commentAction")
		}

		return nil
	})
}

func ProcessComments(ctx *Context) error {
	commentsWriter, err := utility.NewJSONWriter(ctx.Filenames.Output.Comments)
	if err != nil {
		return errors.Wrap(err, "could not create comments writer")
	}
	defer commentsWriter.Close()

	return utility.ReadJSON(ctx.Filenames.Input.Comments, func(line int, data []byte) error {
		// Parse the Comment from the file.
		var in Comment
		if err := easyjson.Unmarshal(data, &in); err != nil {
			logrus.WithField("line", line).Error(err)

			return errors.Wrap(err, "could not parse an comment")
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			return errors.Wrap(err, "checking failed input Action")
		}

		comment := TranslateComment(ctx.TenantID, ctx.SiteID, &in)

		ref, ok := ctx.FindComment(comment.ID)
		if !ok {
			return errors.New("could not find comment ref")
		}

		// Associate the action count data.
		comment.ActionCounts = ref.ActionCounts
		if comment.DeletedAt == nil {
			comment.Revisions[len(comment.Revisions)-1].ActionCounts = ref.ActionCounts
		}

		// Add reconstructed data.
		comment.ChildIDs = ctx.Reconstructor.GetChildren(comment.ID)
		comment.ChildCount = len(comment.ChildIDs)
		comment.AncestorIDs = ctx.Reconstructor.GetAncestors(comment.ID)

		user, _ := ctx.FindOrCreateUser(comment.AuthorID)
		user.StatusCounts.Increment(comment.Status, 1)

		story, _ := ctx.FindOrCreateStory(ref.StoryID)
		story.StatusCounts.Increment(comment.Status, 1)

		// If the comment has at least one flag, count this as a flag on the story
		// reference.
		if comment.ActionCounts["FLAG"] > 0 {
			story.Flagged++
		}

		if err := commentsWriter.Write(comment); err != nil {
			return errors.Wrap(err, "couldn't write out comment")
		}

		return nil
	})
}

func ProcessStories(ctx *Context) error {
	storiesWriter, err := utility.NewJSONWriter(ctx.Filenames.Output.Stories)
	// storiesWriter, err := utility.NewJSONWriter(storiesOutputFilename)
	if err != nil {
		return errors.Wrap(err, "could not create stories writer")
	}
	defer storiesWriter.Close()

	return utility.ReadJSON(ctx.Filenames.Input.Assets, func(line int, data []byte) error {
		// Parse the asset from the file.
		var in Asset
		if err := easyjson.Unmarshal(data, &in); err != nil {
			logrus.WithField("line", line).Error(err)

			return errors.Wrap(err, "could not parse an asset")
		}

		story := TranslateAsset(ctx.TenantID, ctx.SiteID, &in)

		if ref, ok := ctx.FindStory(story.ID); ok {
			// Get the status counts for this story.
			story.CommentCounts.Status = ref.StatusCounts

			// Get the action counts for this story.
			story.CommentCounts.Action = ref.ActionCounts

			// Begin updating the story's moderation queue counts.
			story.CommentCounts.ModerationQueue.Total += story.CommentCounts.Status.None
			story.CommentCounts.ModerationQueue.Total += story.CommentCounts.Status.SystemWithheld
			story.CommentCounts.ModerationQueue.Total += story.CommentCounts.Status.Premod
			story.CommentCounts.ModerationQueue.Queues.Unmoderated += story.CommentCounts.Status.None
			story.CommentCounts.ModerationQueue.Queues.Unmoderated += story.CommentCounts.Status.SystemWithheld
			story.CommentCounts.ModerationQueue.Queues.Unmoderated += story.CommentCounts.Status.Premod
			story.CommentCounts.ModerationQueue.Queues.Pending += story.CommentCounts.Status.Premod
			story.CommentCounts.ModerationQueue.Queues.Pending += story.CommentCounts.Status.SystemWithheld

			// Update the reported queue counts based on the reported map.
			story.CommentCounts.ModerationQueue.Total += ref.Flagged
			story.CommentCounts.ModerationQueue.Queues.Reported += ref.Flagged
		}

		if err := storiesWriter.Write(story); err != nil {
			return errors.Wrap(err, "couldn't write out story")
		}

		return nil
	})
}

func ProcessUsers(ctx *Context) error {
	// usersWriter, err := utility.NewJSONWriter(usersOutputFilename)
	usersWriter, err := utility.NewJSONWriter(ctx.Filenames.Output.Users)
	if err != nil {
		return errors.Wrap(err, "could not create users writer")
	}
	defer usersWriter.Close()

	return utility.ReadJSON(ctx.Filenames.Input.Users, func(line int, data []byte) error {
		// Parse the user from the file.
		var in User
		if err := easyjson.Unmarshal(data, &in); err != nil {
			logrus.WithField("line", line).Error(err)

			return errors.Wrap(err, "could not parse an user")
		}

		user := TranslateUser(ctx.TenantID, &in)

		// Get the status counts for this story.
		if ref, ok := ctx.FindUser(user.ID); ok {
			user.CommentCounts.Status = ref.StatusCounts
		}

		if err := usersWriter.Write(user); err != nil {
			return errors.Wrap(err, "couldn't write out user")
		}

		return nil
	})
}
