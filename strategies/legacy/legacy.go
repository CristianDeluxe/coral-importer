package legacy

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/common/pipeline"
	easyjson "github.com/mailru/easyjson"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var collections = []string{
	"actions",
	"assets",
	"comments",
	"settings",
	"users",
}

// PreferredPerspectiveModel is the stored preferred perspective model that
// should be used to copy over the perspective settings.
var PreferredPerspectiveModel string

// validateCollectionFilesExist will ensure that all the collection files that
// we expect to be in the input directory actually exist.
func validateCollectionFilesExist(input string) error {
	for _, collection := range collections {
		filePath := filepath.Join(input, fmt.Sprintf("%s.json", collection))
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return errors.Errorf("%s does not exist", filePath)
		}
	}

	return nil
}

// Import will handle a data import task for importing comments into Coral from
// a legacy export.
func Import(c *cli.Context) error {
	// Copy over the preferredPerspectiveModel from the flags.
	PreferredPerspectiveModel = c.String("preferredPerspectiveModel")

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

	// Validate that the collection files we expect exist in the input folder.
	if err := validateCollectionFilesExist(input); err != nil {
		return err
	}

	// Mark when we started.
	started := time.Now()

	// Load all the comment actions from the actions.json file.
	actionsFileName := filepath.Join(input, "actions.json")
	commentsFileName := filepath.Join(input, "comments.json")

	commentMap, err := pipeline.NewMapAggregator(
		pipeline.MergeTaskAggregatorOutputPipelines(
			pipeline.FanAggregatingProcessor(
				pipeline.NewJSONFileReader(commentsFileName),
				ProcessCommentMap(),
			),
		),
	)
	if err != nil {
		return errors.Wrap(err, "could not process comments")
	}

	logrus.WithField("comments", len(commentMap["storyID"])).WithField("children", len(commentMap["parentID"])).Debug("finished loading comments into map")

	// Write out all the commentActions to ${output}/commentActions.json.
	if err := pipeline.NewFileWriter(
		output,
		pipeline.MergeTaskWriterOutputPipelines(
			pipeline.FanWritingProcessors(
				pipeline.NewJSONFileReader(actionsFileName),
				ProcessCommentActions(tenantID, siteID, commentMap["storyID"]),
			),
		),
	); err != nil {
		return errors.Wrap(err, "could not process comment actions")
	}

	logrus.Debug("finished writing out comment actions")

	// Load all the action counts using our summer.
	actionCountsFileName := filepath.Join(output, "commentActions.json")
	actionCounts, err := pipeline.NewSummer(
		pipeline.MergeTaskSummerOutputPipelines(
			pipeline.FanSummerProcessor(
				pipeline.NewJSONFileReader(actionCountsFileName),
				ProcessActionCounts(),
			),
		),
	)
	if err != nil {
		return errors.Wrap(err, "could not process action counts")
	}

	logrus.Debug("finished calculating comment action counts")

	startedReconstructionAt := time.Now()
	logrus.Debug("reconstructing families based on parent id map")

	// Reconstruct all the family relationships from the parentID map.
	reconstructor := common.NewReconstructor()
	for commentID, parentIDs := range commentMap["parentID"] {
		if len(parentIDs) == 1 {
			reconstructor.AddIDs(commentID, parentIDs[0])
		} else {
			reconstructor.AddIDs(commentID, "")
		}
	}

	logrus.WithField("took", time.Since(startedReconstructionAt).String()).Debug("finished family reconstruction")

	// Delete the reference to the parentID map that we don't need any more.
	delete(commentMap, "parentID")

	// Load all the comments in from the comments.json file.
	if err := pipeline.NewFileWriter(
		output,
		pipeline.MergeTaskWriterOutputPipelines(
			pipeline.FanWritingProcessors(
				pipeline.NewJSONFileReader(commentsFileName),
				ProcessComments(tenantID, siteID, actionCounts, reconstructor),
			),
		),
	); err != nil {
		return errors.Wrap(err, "could not process comments")
	}

	// Load all the comment statuses by reading the comments.json file again.
	statusCounts, err := pipeline.NewSummer(
		pipeline.MergeTaskSummerOutputPipelines(
			pipeline.FanSummerProcessor(
				pipeline.NewJSONFileReader(commentsFileName),
				ProcessCommentStatusMap(),
			),
		),
	)
	if err != nil {
		return errors.Wrap(err, "could not process status counts")
	}

	// Walk across all the comment's status maps so we can determine how many
	// comments should be in the reported queue in each story.
	reportedMap := make(map[string]int)
	for commentID, statues := range commentMap["status"] {
		if len(statues) != 1 {
			continue
		}

		if statues[0] != "NONE" {
			// If the status is not none, then it can't be reported!
			continue
		}

		commentActionCounts, ok := actionCounts[commentID]
		if !ok {
			// The comment didn't have any action counts, so it can't be
			// flagged, so keep going!
			continue
		}

		flagCount := commentActionCounts["FLAG"]
		if flagCount > 0 {
			// Get this comment's story ID.
			storyIDs := commentMap["storyID"][commentID]
			if len(storyIDs) != 1 {
				continue
			}

			// Increment the storyID.
			reportedMap[storyIDs[0]]++
		}
	}

	// Process the stories now.
	assetsFileName := filepath.Join(input, "assets.json")
	if err := pipeline.NewFileWriter(
		output,
		pipeline.MergeTaskWriterOutputPipelines(
			pipeline.FanWritingProcessors(
				pipeline.NewJSONFileReader(assetsFileName),
				ProcessStories(tenantID, siteID, statusCounts, actionCounts, reportedMap),
			),
		),
	); err != nil {
		return errors.Wrap(err, "could not process stories")
	}

	// Write out all the users to ${output}/users.json.
	usersFileName := filepath.Join(input, "users.json")
	if err := pipeline.NewFileWriter(
		output,
		pipeline.MergeTaskWriterOutputPipelines(
			pipeline.FanWritingProcessors(
				pipeline.NewJSONFileReader(usersFileName),
				ProcessUsers(tenantID, statusCounts),
			),
		),
	); err != nil {
		return errors.Wrap(err, "could not process users")
	}

	// Mark when we finished.
	finished := time.Now()
	logrus.WithField("took", finished.Sub(started).String()).Info("finished processing")

	return nil
}

func ProcessUsers(tenantID string, statusCounts map[string]map[string]int) pipeline.WritingProcessor {
	return func(write pipeline.CollectionWriter, n *pipeline.TaskReaderInput) error {
		// Parse the user from the file.
		var in User
		if err := easyjson.Unmarshal([]byte(n.Input), &in); err != nil {
			logrus.WithField("line", n.Line).Error(err)

			return errors.Wrap(err, "could not parse an user")
		}

		user := TranslateUser(tenantID, &in)

		// Get the status counts for this story.
		userStatusCounts := statusCounts["user:"+user.ID]
		user.CommentCounts.Status.Approved = userStatusCounts["APPROVED"]
		user.CommentCounts.Status.None = userStatusCounts["NONE"]
		user.CommentCounts.Status.Premod = userStatusCounts["PREMOD"]
		user.CommentCounts.Status.Rejected = userStatusCounts["REJECTED"]
		user.CommentCounts.Status.SystemWithheld = userStatusCounts["SYSTEM_WITHHELD"]

		if err := write("users", user); err != nil {
			return errors.Wrap(err, "couldn't write out user")
		}

		return nil
	}
}

func ProcessStories(tenantID, siteID string, statusCounts, actionCounts map[string]map[string]int, reportedMap map[string]int) pipeline.WritingProcessor {
	return func(write pipeline.CollectionWriter, n *pipeline.TaskReaderInput) error {
		// Parse the asset from the file.
		var in Asset
		if err := easyjson.Unmarshal([]byte(n.Input), &in); err != nil {
			logrus.WithField("line", n.Line).Error(err)

			return errors.Wrap(err, "could not parse an asset")
		}

		story := TranslateAsset(tenantID, siteID, &in)

		// Get the status counts for this story.
		storyStatusCounts := statusCounts["story:"+story.ID]
		story.CommentCounts.Status.Approved = storyStatusCounts["APPROVED"]
		story.CommentCounts.Status.None = storyStatusCounts["NONE"]
		story.CommentCounts.Status.Premod = storyStatusCounts["PREMOD"]
		story.CommentCounts.Status.Rejected = storyStatusCounts["REJECTED"]
		story.CommentCounts.Status.SystemWithheld = storyStatusCounts["SYSTEM_WITHHELD"]

		// Get the action counts for this story.
		storyActionCounts := actionCounts[story.ID]
		if storyActionCounts == nil {
			storyActionCounts = map[string]int{}
		}
		story.CommentCounts.Action = storyActionCounts

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
		story.CommentCounts.ModerationQueue.Total += reportedMap[story.ID]
		story.CommentCounts.ModerationQueue.Queues.Reported += reportedMap[story.ID]

		if err := write("stories", story); err != nil {
			return errors.Wrap(err, "couldn't write out story")
		}

		return nil
	}
}

func ProcessCommentStatusMap() pipeline.SummerProcessor {
	return func(writer pipeline.SummerWriter, n *pipeline.TaskReaderInput) error {
		// Parse the comment from the file.
		var in Comment
		if err := easyjson.Unmarshal([]byte(n.Input), &in); err != nil {
			logrus.WithField("line", n.Line).Error(err)

			return errors.Wrap(err, "could not parse an comment")
		}

		// Get the comment status, translated.
		status := TranslateCommentStatus(in.Status)

		// Add the status to the map referencing the story id.
		writer("story:"+in.AssetID, status, 1)

		// Add the status to the map referencing the user id.
		writer("user:"+in.AuthorID, status, 1)

		return nil
	}
}

func ProcessComments(tenantID, siteID string, actionCounts map[string]map[string]int, r *common.Reconstructor) pipeline.WritingProcessor {
	return func(write pipeline.CollectionWriter, n *pipeline.TaskReaderInput) error {
		// Parse the Comment from the file.
		var in Comment
		if err := easyjson.Unmarshal([]byte(n.Input), &in); err != nil {
			logrus.WithField("line", n.Line).Error(err)

			return errors.Wrap(err, "could not parse an comment")
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			return errors.Wrap(err, "checking failed input Action")
		}

		comment := TranslateComment(tenantID, siteID, &in)

		commentActionCounts, ok := actionCounts[comment.ID]
		if !ok {
			commentActionCounts = map[string]int{}
		}

		// Associate the action count data.
		comment.ActionCounts = commentActionCounts
		if comment.DeletedAt == nil {
			comment.Revisions[len(comment.Revisions)-1].ActionCounts = commentActionCounts
		}

		// Add reconstructed data.
		comment.ChildIDs = r.GetChildren(comment.ID)
		comment.ChildCount = len(comment.ChildIDs)
		comment.AncestorIDs = r.GetAncestors(comment.ID)

		if err := write("comments", comment); err != nil {
			return errors.Wrap(err, "couldn't write out comment")
		}

		return nil
	}
}

func ProcessActionCounts() pipeline.SummerProcessor {
	return func(writer pipeline.SummerWriter, n *pipeline.TaskReaderInput) error {
		// Parse the comment actions.
		var in coral.CommentAction
		if err := easyjson.Unmarshal([]byte(n.Input), &in); err != nil {
			logrus.WithField("line", n.Line).Error(err)

			return errors.Wrap(err, "could not parse an action")
		}

		// Write out the sums for this particular comment action.
		writer(in.CommentID, in.ActionType, 1)
		writer(in.StoryID, in.ActionType, 1)
		if in.ActionType == "FLAG" {
			writer(in.CommentID, in.ActionType+"__"+in.Reason, 1)
			writer(in.StoryID, in.ActionType+"__"+in.Reason, 1)
		}

		return nil
	}
}

func ProcessCommentMap() pipeline.AggregatingProcessor {
	return func(writer pipeline.AggregationWriter, input *pipeline.TaskReaderInput) error {
		// Parse the comment from the file.
		var in Comment
		if err := easyjson.Unmarshal([]byte(input.Input), &in); err != nil {
			logrus.WithField("line", input.Line).Error(err)

			return errors.Wrap(err, "could not parse a comment")
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			logrus.WithError(err).WithField("line", input.Line).Warn("validation failed for input user")

			return nil
		}

		// Write the comment story ID out to the map.
		writer("status", in.ID, TranslateCommentStatus(in.Status))
		writer("storyID", in.ID, in.AssetID)
		if in.ParentID != nil {
			writer("parentID", in.ID, *in.ParentID)
		} else {
			writer("parentID", in.ID, "")
		}

		return nil
	}
}

func ProcessCommentActions(tenantID, siteID string, comments map[string][]string) pipeline.WritingProcessor {
	return func(write pipeline.CollectionWriter, input *pipeline.TaskReaderInput) error {
		// Parse the Action from the file.
		var in Action
		if err := easyjson.Unmarshal([]byte(input.Input), &in); err != nil {
			logrus.WithField("line", input.Line).Error(err)

			return errors.Wrap(err, "could not parse an action")
		}

		// Ignore the action if it's not a comment action.
		if in.ItemType != "COMMENTS" {
			logrus.WithField("line", input.Line).Warn("skipping non-comment flag")

			return nil
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			return errors.Wrap(err, "checking failed input Action")
		}

		// Translate the action to a comment action.
		action := TranslateCommentAction(tenantID, siteID, &in)
		storyID, ok := comments[action.CommentID]
		if !ok || len(storyID) != 1 {
			return nil
		}
		action.StoryID = storyID[0]

		if err := write("commentActions", action); err != nil {
			return errors.Wrap(err, "couldn't write out commentAction")
		}

		return nil
	}
}
