package livefyre

import (
	"fmt"
	"time"

	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/common/pipeline"
	easyjson "github.com/mailru/easyjson"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func ProcessUsersMap() pipeline.AggregatingProcessor {
	return func(writer pipeline.AggregationWriter, input *pipeline.TaskReaderInput) error {
		// Parse the User from the file.
		var in User
		if err := easyjson.Unmarshal([]byte(input.Input), &in); err != nil {
			return errors.Wrap(err, "could not parse a user in the --users file")
		}

		// Check the input to ensure we're validated.
		if err := common.Check(&in); err != nil {
			logrus.WithError(err).WithField("line", input.Line).Warn("validation failed for input user")

			return nil
		}

		// Write the user details out to the writer.
		writer("id", in.Email, in.ID)
		writer("display_name", in.Email, in.DisplayName)

		return nil
	}
}

func ProcessUsers(tenantID string, sso bool, users map[string]map[string][]string, statusCounts map[string]map[string]int) <-chan pipeline.TaskWriterOutput {
	out := make(chan pipeline.TaskWriterOutput)
	go func() {
		defer close(out)

		now := time.Now()

		for email, displayNames := range users["display_name"] {
			// Grab this User's ID's.
			id := users["id"][email][0]

			// See if the user has even one display name.
			if len(displayNames) == 0 {
				displayNames = []string{
					fmt.Sprintf("User %s", id),
				}
			}

			// Build a coral.User from the user information we have.
			user := TranslateUser(tenantID, &User{
				ID:          id,
				Email:       email,
				DisplayName: displayNames[0],
			}, now)
			if sso {
				user.Profiles = append(user.Profiles, coral.UserProfile{

					ID:           user.ID,
					Type:         "sso",
					LastIssuedAt: &user.CreatedAt,
				})
			}

			// Get the status counts for this user.
			userStatusCounts := statusCounts[user.ID]
			user.CommentCounts.Status.Approved = userStatusCounts["APPROVED"]
			user.CommentCounts.Status.None = userStatusCounts["NONE"]
			user.CommentCounts.Status.Premod = userStatusCounts["PREMOD"]
			user.CommentCounts.Status.Rejected = userStatusCounts["REJECTED"]
			user.CommentCounts.Status.SystemWithheld = userStatusCounts["SYSTEM_WITHHELD"]

			// Serialize the user for output.
			doc, err := easyjson.Marshal(user)
			if err != nil {
				out <- pipeline.TaskWriterOutput{
					Error: errors.Wrap(err, "could not marshal the coral.User"),
				}

				return
			}

			logrus.WithFields(logrus.Fields{
				"userID": user.ID,
			}).Debug("imported user")

			// Write the user out.
			out <- pipeline.TaskWriterOutput{
				Collection: "users",
				Document:   doc,
			}
		}
	}()

	return out
}
