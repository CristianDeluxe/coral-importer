package livefyre

import (
	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/common/pipeline"
	easyjson "github.com/mailru/easyjson"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

func ProcessComments(tenantID, siteID string, authorIDs map[string]string) pipeline.WritingProcessor {
	return func(write pipeline.CollectionWriter, n *pipeline.TaskReaderInput) error {
		// Parse the Story from the file.
		var in Story
		if err := easyjson.Unmarshal([]byte(n.Input), &in); err != nil {
			return errors.Wrap(err, "could not parse a comment in the --input file")
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			logrus.WithField("line", n.Line).WithError(err).Error("cannot validate story")

			return errors.Wrap(err, "checking failed input Story")
		}

		// Translate the Story to a coral.Story.
		story := TranslateStory(tenantID, siteID, &in)

		// Check the story to ensure we're validated.
		if err := common.Check(story); err != nil {
			return errors.Wrap(err, "checking failed output coral.Story")
		}

		// Collect all the stories comments so we can process family
		// relationships as well.
		storyComments := make([]*coral.Comment, 0, len(in.Comments))

		// Reconstruct family relationships for these comments.
		r := common.NewReconstructor()

		// Store the reaction total for the story.
		storyReactionTotal := 0

		// Translate the comments.
		for i, inc := range in.Comments {
			if inc.AuthorID == "" {
				logrus.WithFields(logrus.Fields{
					"storyID":   story.ID,
					"commentID": inc.ID,
					"line":      n.Line,
				}).Warn("comment was missing author_id field")

				continue
			}

			// Check the comment to ensure we're validated.
			if err := common.Check(&in.Comments[i]); err != nil {
				return errors.Wrapf(err, "checking failed input Comment for Story %s", story.ID)
			}

			// Remap the authorID.
			authorID := authorIDs[inc.AuthorID]
			if authorID == "" {
				logrus.WithFields(logrus.Fields{
					"storyID":   story.ID,
					"commentID": inc.ID,
					"authorID":  inc.AuthorID,
					"line":      n.Line,
				}).Warn("comment author_id did not exist in author map")

				continue
			}
			inc.AuthorID = authorID

			// Translate the Comment to a coral.Comment.
			comment := TranslateComment(tenantID, siteID, &in.Comments[i])
			comment.StoryID = story.ID

			// Check the comment to ensure we're validated.
			if err := common.Check(comment); err != nil {
				return errors.Wrap(err, "checking failed output coral.Comment")
			}

			// Add the comment to the reconstructor.
			r.AddComment(comment)

			// Look at the comment to see if there are any likes on it.
			if inc.Likes != nil {
				reactionTotal := 0
				for _, likeUserID := range inc.Likes {
					// Remap the like user ID to the one from the author map. If
					// we can't remap the user id it means we don't have a user
					// for this like, and therefore it shouldn't be imported
					// either.
					mappedLikeUserID := authorIDs[likeUserID]
					if mappedLikeUserID == "" {
						logrus.WithFields(logrus.Fields{
							"storyID":   story.ID,
							"commentID": inc.ID,
							"like":      likeUserID,
							"line":      n.Line,
						}).Warn("could not find user ID of like in author map, not importing like")

						continue
					}

					// Create a new Comment Action for this like.
					action := coral.NewCommentAction(tenantID, siteID)
					action.ID = uuid.NewV4().String()
					action.ActionType = "REACTION"
					action.CommentID = comment.ID
					action.UserID = &mappedLikeUserID
					action.CommentRevisionID = comment.ID
					action.StoryID = story.ID

					// Check the action to ensure we're validated.
					if err := common.Check(action); err != nil {
						return errors.Wrap(err, "checking failed output coral.CommentAction")
					}

					if err := write("commentActions", action); err != nil {
						return errors.Wrap(err, "couldn't write out commentAction")
					}

					logrus.WithFields(logrus.Fields{
						"storyID":   story.ID,
						"commentID": comment.ID,
						"line":      n.Line,
					}).Debug("imported reaction")

					reactionTotal++
				}

				// Add the reaction count to the comment.
				comment.ActionCounts["REACTION"] = reactionTotal
				storyReactionTotal += reactionTotal
			}

			// Add it to the story comments.
			storyComments = append(storyComments, comment)
		}

		if len(storyComments) == 0 {
			logrus.WithFields(logrus.Fields{
				"storyID": story.ID,
				"line":    n.Line,
			}).Warn("no comments imported from story")
		}

		// Send the comments off to the importer.
		for _, comment := range storyComments {
			// Add reconstructed data.
			comment.ChildIDs = r.GetChildren(comment.ID)
			comment.ChildCount = len(comment.ChildIDs)
			comment.AncestorIDs = r.GetAncestors(comment.ID)

			// Send the comment to the importer.
			if err := write("comments", comment); err != nil {
				return errors.Wrap(err, "couldn't write out comment")
			}

			logrus.WithFields(logrus.Fields{
				"storyID":   story.ID,
				"commentID": comment.ID,
				"line":      n.Line,
			}).Debug("imported comment")
		}

		// Increment the stories comment counts.
		for _, comment := range storyComments {
			story.IncrementCommentCounts(comment.Status)
		}
		story.CommentCounts.Action["REACTION"] = storyReactionTotal

		// Send the story to the importer.
		if err := write("stories", story); err != nil {
			return errors.Wrap(err, "couldn't write out story")
		}

		logrus.WithFields(logrus.Fields{
			"storyID": story.ID,
			"line":    n.Line,
		}).Debug("imported story")

		logrus.WithFields(logrus.Fields{
			"storyID":  story.ID,
			"line":     n.Line,
			"comments": len(storyComments),
		}).Info("finished line")

		return nil
	}
}

func ProcessCommentStatusMap() pipeline.SummerProcessor {
	return func(writer pipeline.SummerWriter, n *pipeline.TaskReaderInput) error {
		// Parse the comment from the file.
		var comment coral.Comment
		if err := easyjson.Unmarshal([]byte(n.Input), &comment); err != nil {
			return errors.Wrap(err, "could not parse an comment")
		}

		// Add the status to the map referencing the user id.
		writer(comment.AuthorID, comment.Status, 1)

		return nil
	}
}
