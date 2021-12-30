//go:generate easyjson -all models.go
package legacy

import (
	"fmt"
	"strings"

	"coral-importer/common/coral"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// Action is the Action as exported from MongoDB from legacy Talk.
type Action struct {
	ID         string                 `json:"id"`
	ActionType string                 `json:"action_type"`
	GroupID    string                 `json:"group_id"`
	ItemID     string                 `json:"item_id"`
	ItemType   string                 `json:"item_type"`
	UserID     *string                `json:"user_id"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  coral.Time             `json:"created_at"`
}

func TranslateCommentAction(tenantID, siteID string, action *Action) *coral.CommentAction {
	commentAction := coral.NewCommentAction(tenantID, siteID)
	commentAction.ID = action.ID

	switch action.ActionType {
	case "FLAG":
		commentAction.ActionType = "FLAG"
		switch action.GroupID {
		case "BANNED_WORD":
			commentAction.Reason = "COMMENT_DETECTED_BANNED_WORD"
		case "COMMENT_OFFENSIVE":
			commentAction.Reason = "COMMENT_REPORTED_OFFENSIVE"
		case "COMMENT_OTHER":
			commentAction.Reason = "COMMENT_REPORTED_OTHER"
		case "COMMENT_SPAM":
			commentAction.Reason = "COMMENT_REPORTED_SPAM"
		case "LINKS":
			commentAction.Reason = "COMMENT_DETECTED_LINKS"
		case "SPAM_COMMENT":
			commentAction.Reason = "COMMENT_DETECTED_SPAM"
		case "TRUST":
			commentAction.Reason = "COMMENT_DETECTED_RECENT_HISTORY"
		case "TOXIC_COMMENT":
			commentAction.Reason = "COMMENT_DETECTED_TOXIC"
		case "":
		default:
		}

		if action.Metadata != nil {
			message, ok := action.Metadata["message"].(string)
			if ok {
				commentAction.AdditionalDetails = message
			}
		}
	case "DONTAGREE":
		commentAction.ActionType = "DONT_AGREE"
	default:
		commentAction.ActionType = "REACTION"
	}

	commentAction.CommentID = action.ItemID
	commentAction.UserID = action.UserID
	commentAction.CreatedAt.Time = action.CreatedAt.Time

	// v4 did not have revision ID's, so use the comment ID which will always be
	// used as the most recent body history item anyways when we import the
	// comments.
	commentAction.CommentRevisionID = action.ItemID

	// The following must be processed when we've loaded all the comments in via
	// a second pass.
	commentAction.StoryID = ""

	return commentAction
}

type CommentBodyHistory struct {
	Body      string     `json:"body"`
	CreatedAt coral.Time `json:"created_at"`
}

type CommentTag struct {
	AssignedBy *string `json:"assigned_by"`
	Tag        struct {
		Name string `json:"name"`
	} `json:"tag"`
	CreatedAt coral.Time `json:"created_at"`
}

type Comment struct {
	ID            string `json:"id"`
	AssetID       string `json:"asset_id"`
	Status        string `json:"status"`
	StatusHistory []struct {
		AssignedBy *string    `json:"assigned_by"`
		Type       string     `json:"type"`
		CreatedAt  coral.Time `json:"created_at"`
	} `json:"status_history"`
	Metadata *struct {
		Perspective map[string]struct {
			SummaryScore float64 `json:"summaryScore"`
		} `json:"perspective"`
		Akismet *bool `json:"akismet"`
	} `json:"metadata"`
	BodyHistory []CommentBodyHistory `json:"body_history"`
	Tags        []CommentTag         `json:"tags"`
	ParentID    *string              `json:"parent_id"`
	AuthorID    string               `json:"author_id"`
	DeletedAt   *coral.Time          `json:"deleted_at"`
	CreatedAt   coral.Time           `json:"created_at"`
	UpdatedAt   coral.Time           `json:"updated_at"`
}

func TranslateCommentStatus(status string) string {
	if status == "ACCEPTED" {
		return "APPROVED"
	}

	return status
}

func TranslateComment(tenantID, siteID string, in *Comment) *coral.Comment {
	comment := coral.NewComment(tenantID, siteID)
	comment.ID = in.ID

	if in.ParentID != nil {
		comment.ParentID = *in.ParentID
		comment.ParentRevisionID = *in.ParentID
	}

	comment.AuthorID = in.AuthorID
	comment.CreatedAt.Time = in.CreatedAt.Time
	comment.StoryID = in.AssetID
	comment.Status = TranslateCommentStatus(in.Status)
	for _, tag := range in.Tags {
		// If the tag name is not STAFF or FEATURED, then don't add it to the tags
		// array! Coral Modern does not support any other tag from Coral Legacy.
		if tag.Tag.Name != "STAFF" && tag.Tag.Name != "FEATURED" {
			continue
		}

		commentTag := coral.CommentTag{
			Type:      tag.Tag.Name,
			CreatedAt: tag.CreatedAt,
		}

		if tag.AssignedBy != nil {
			commentTag.CreatedBy = *tag.AssignedBy
		}

		comment.Tags = append(comment.Tags, commentTag)
	}

	if in.DeletedAt == nil {
		revisionLength := len(in.BodyHistory)

		if revisionLength <= 0 {
			panic(fmt.Sprintf("%s with deletedAt: %v which is %v", in.ID, in.DeletedAt, in.DeletedAt == nil))
		}

		comment.Revisions = make([]coral.Revision, revisionLength)
		for i, revision := range in.BodyHistory {
			// The body that comes from the revision body will contain `\n`
			// characters. We need to convert these to `<br>` tags.
			body := strings.ReplaceAll(revision.Body, "\n", "<br>")

			comment.Revisions[i] = coral.Revision{
				ID:           comment.ID + "-" + fmt.Sprintf("%d", i),
				Body:         coral.HTML(body),
				Metadata:     coral.RevisionMetadata{},
				ActionCounts: map[string]int{},
			}
			comment.Revisions[i].CreatedAt.Time = revision.CreatedAt.Time
		}

		// Ensure that the last revision ID is the comment's ID.
		comment.Revisions[revisionLength-1].ID = comment.ID

		// Copy over the revision metadata for the last revision if it's set.
		if in.Metadata != nil {
			if in.Metadata.Perspective != nil {
				// Try to get the perspective model from the map.
				model := PreferredPerspectiveModel
				perspective, ok := in.Metadata.Perspective[model]
				if !ok {
					// A perspective model was not found, get the first one it
					// has and break out of the loop.
					for modelName, scores := range in.Metadata.Perspective {
						ok = true
						model = modelName
						perspective.SummaryScore = scores.SummaryScore

						break
					}
				}

				// If a perspective model was found, then set it.
				if ok {
					comment.Revisions[revisionLength-1].Metadata.Perspective = &coral.RevisionPerspective{
						Score: perspective.SummaryScore,
						Model: model,
					}
				}
			}

			// If the akismet values are provided, then set it.
			if in.Metadata.Akismet != nil {
				comment.Revisions[revisionLength-1].Metadata.Akismet = in.Metadata.Akismet
			}
		}
	} else {
		comment.DeletedAt = in.DeletedAt
	}

	// Attach extra data of the
	comment.Extra["status_history"] = in.StatusHistory

	return comment
}

type Asset struct {
	ID            string      `json:"id"`
	URL           string      `json:"url"`
	ClosedAt      *coral.Time `json:"closedAt"`
	ClosedMessage *string     `json:"closedMessage"`
	CreatedAt     coral.Time  `json:"created_at"`
	Scraped       *coral.Time `json:"scraped"`
	Metadata      interface{} `json:"metadata"`
	Title         *string     `json:"title"`
	Author        *string     `json:"author"`
	Description   *string     `json:"description"`
	Image         *string     `json:"image"`
	Section       *string     `json:"section"`
	Settings      struct {
		Moderation         *string `json:"moderation,omitempty"`
		QuestionBoxContent *string `json:"questionBoxContent,omitempty"`
		QuestionBoxEnable  *bool   `json:"questionBoxEnable,omitempty"`
		QuestionBoxIcon    *string `json:"questionBoxIcon,omitempty"`
	} `json:"settings"`
	ModifiedDate    *coral.Time `json:"modified_date"`
	PublicationDate *coral.Time `json:"publication_date"`
}

func TranslateAsset(tenantID, siteID string, asset *Asset) *coral.Story {
	story := coral.NewStory(tenantID, siteID)
	story.ID = asset.ID
	story.URL = asset.URL

	if asset.Title != nil {
		story.Metadata.Title = *asset.Title
	}

	if asset.Author != nil {
		story.Metadata.Author = *asset.Author
	}

	if asset.Description != nil {
		story.Metadata.Description = *asset.Description
	}

	if asset.Image != nil {
		story.Metadata.Image = *asset.Image
	}

	if asset.Section != nil {
		story.Metadata.Section = *asset.Section
	}

	if asset.PublicationDate != nil {
		story.Metadata.PublishedAt = asset.PublicationDate
	}

	story.CreatedAt = asset.CreatedAt

	if asset.ClosedAt != nil {
		story.ClosedAt = asset.ClosedAt
	}

	if asset.Settings.Moderation != nil {
		story.Settings.Moderation = asset.Settings.Moderation
	}

	if asset.Settings.QuestionBoxEnable != nil && asset.Settings.QuestionBoxContent != nil {
		story.Settings.MessageBox = &coral.MessageBox{
			Enabled: *asset.Settings.QuestionBoxEnable,
			Content: *asset.Settings.QuestionBoxContent,
		}
	}

	return story
}

type UserProfile struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
}

type UserToken struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type UserNotificationSettings struct {
	OnFeatured      *bool       `json:"onFeatured"`
	OnModeration    *bool       `json:"onModeration"`
	OnReply         *bool       `json:"onReply"`
	OnStaffReply    *bool       `json:"onStaffReply"`
	DigestFrequency interface{} `json:"digestFrequency"`
}

type UserNotifications struct {
	Settings *UserNotificationSettings `json:"settings"`
}

type UserMetadata struct {
	Notifications       *UserNotifications `json:"notifications"`
	LastAccountDownload *coral.Time        `json:"lastAccountDownload"`
	DisplayName         string             `json:"displayName"`
}

type User struct {
	ID           string        `json:"id"`
	Username     string        `json:"username"`
	Role         string        `json:"role"`
	Password     string        `json:"password"`
	IgnoredUsers []string      `json:"ignoresUsers"`
	Profiles     []UserProfile `json:"profiles"`
	Tokens       []UserToken   `json:"tokens"`
	Status       struct {
		Username struct {
			Status  string `json:"status"`
			History []struct {
				AssignedBy *string    `json:"assigned_by"`
				Status     string     `json:"status"`
				CreatedAt  coral.Time `json:"created_at"`
			} `json:"history"`
		} `json:"username"`
		Banned struct {
			Status  bool `json:"status"`
			History []struct {
				AssignedBy *string    `json:"assigned_by"`
				Message    string     `json:"message"`
				Status     bool       `json:"status"`
				CreatedAt  coral.Time `json:"created_at"`
			} `json:"history"`
		} `json:"banned"`
		Suspension struct {
			Until   *coral.Time `json:"until"`
			History []struct {
				AssignedBy *string    `json:"assigned_by"`
				Message    string     `json:"message"`
				Until      coral.Time `json:"until"`
				CreatedAt  coral.Time `json:"created_at"`
			} `json:"history"`
		} `json:"suspension"`
		AlwaysPremod struct {
			Status  bool `json:"status"`
			History []struct {
				AssignedBy string     `json:"assigned_by"`
				Status     bool       `json:"status"`
				CreatedAt  coral.Time `json:"created_at"`
			} `json:"history"`
		} `json:"alwaysPremod"`
	} `json:"status"`
	CreatedAt coral.Time    `json:"created_at"`
	Metadata  *UserMetadata `json:"metadata"`
}

func TranslateUserProfile(user *coral.User, in *User, profile UserProfile) *coral.UserProfile {
	switch profile.Provider {
	case "local":
		user.Email = strings.ToLower(profile.ID)

		return &coral.UserProfile{
			ID:         user.Email,
			Type:       "local",
			Password:   in.Password,
			PasswordID: uuid.NewV4().String(),
		}
	case "facebook":
		return &coral.UserProfile{
			ID:   profile.ID,
			Type: "facebook",
		}
	case "google":
		return &coral.UserProfile{
			ID:   profile.ID,
			Type: "google",
		}
	default:
		logrus.WithField("provider", profile.Provider).Warn("unsupported provider not imported")

		return nil
	}
}

func TranslateUser(tenantID string, in *User) *coral.User {
	user := coral.NewUser(tenantID)
	user.ID = in.ID
	user.Role = in.Role
	user.CreatedAt = in.CreatedAt
	user.IgnoredUsers = make([]coral.IgnoredUser, len(in.IgnoredUsers))
	for i, ignoredUserID := range in.IgnoredUsers {
		user.IgnoredUsers[i] = coral.IgnoredUser{
			ID:        ignoredUserID,
			CreatedAt: user.CreatedAt,
		}
	}
	if len(in.Profiles) > 0 {
		user.Profiles = make([]coral.UserProfile, 0, len(in.Profiles))
		for _, profile := range in.Profiles {
			profile := TranslateUserProfile(user, in, profile)
			if profile != nil {
				user.Profiles = append(user.Profiles, *profile)
			}
		}
	}

	user.Status.SuspensionStatus.History = make([]coral.UserSuspensionStatusHistory, len(in.Status.Suspension.History))
	for i, history := range in.Status.Suspension.History {
		user.Status.SuspensionStatus.History[i] = coral.UserSuspensionStatusHistory{
			ID: uuid.NewV1().String(),
			From: coral.TimeRange{
				Start:  history.CreatedAt,
				Finish: history.Until,
			},
			Message:   history.Message,
			CreatedAt: history.CreatedAt,
		}

		if history.AssignedBy != nil {
			user.Status.SuspensionStatus.History[i].CreatedBy = *history.AssignedBy
		}
	}
	//set user.username for username status history, if a metadata.displayName value is present this will be overwritten 
	user.Username = in.Username
	user.Status.BanStatus.Active = in.Status.Banned.Status
	user.Status.BanStatus.History = make([]coral.UserBanStatusHistory, len(in.Status.Banned.History))
	for i, history := range in.Status.Banned.History {
		user.Status.BanStatus.History[i] = coral.UserBanStatusHistory{
			ID:        uuid.NewV1().String(),
			Message:   history.Message,
			Active:    history.Status,
			CreatedAt: history.CreatedAt,
		}

		if history.AssignedBy != nil {
			user.Status.BanStatus.History[i].CreatedBy = *history.AssignedBy
		}
	}

	user.Status.PremodStatus.Active = in.Status.AlwaysPremod.Status
	if len(in.Status.AlwaysPremod.History) > 0 {
		user.Status.PremodStatus.History = make([]coral.UserPremodStatusHistory, len(in.Status.AlwaysPremod.History))
		for i, status := range in.Status.AlwaysPremod.History {
			user.Status.PremodStatus.History[i] = coral.UserPremodStatusHistory{
				AssignedBy: status.AssignedBy,
				Status:     status.Status,
				CreatedAt:  status.CreatedAt,
			}
		}
	}

	user.Status.UsernameStatus.History = []coral.UserUsernameStatusHistory{
		{
			ID:        uuid.NewV1().String(),
			Username:  user.Username,
			CreatedBy: user.ID,
			CreatedAt: user.CreatedAt,
		},
	}

	for _, token := range in.Tokens {
		if token.Active {
			user.Tokens = append(user.Tokens, coral.UserToken{
				ID:        token.ID,
				Name:      token.Name,
				CreatedAt: user.CreatedAt,
			})
		}
	}

	if in.Metadata != nil {
		// Handle value of metadata.displayName if present, otherwise username was set during username status history 
		if in.Metadata.DisplayName != "" {
			user.Username = in.Metadata.DisplayName
		} 
		if in.Metadata.Notifications != nil && in.Metadata.Notifications.Settings != nil {
			if in.Metadata.Notifications.Settings.OnReply != nil {
				user.Notifications.OnReply = *in.Metadata.Notifications.Settings.OnReply
			}

			if in.Metadata.Notifications.Settings.OnFeatured != nil {
				user.Notifications.OnFeatured = *in.Metadata.Notifications.Settings.OnFeatured
			}

			if in.Metadata.Notifications.Settings.OnStaffReply != nil {
				user.Notifications.OnStaffReplies = *in.Metadata.Notifications.Settings.OnStaffReply
			}

			if in.Metadata.Notifications.Settings.OnModeration != nil {
				user.Notifications.OnModeration = *in.Metadata.Notifications.Settings.OnModeration
			}

			if in.Metadata.Notifications.Settings.DigestFrequency != nil {
				// Sometimes it seems the digestFrequency is `false` instead of a
				// string, this is a mitigation for that.
				if digestFrequency, ok := in.Metadata.Notifications.Settings.DigestFrequency.(string); ok {
					user.Notifications.DigestFrequency = digestFrequency
				} else {
					user.Notifications.DigestFrequency = "NONE"
				}
			} else {
				user.Notifications.DigestFrequency = "NONE"
			}
		}

		// Assign the last downloaded at time stamp.
		user.LastDownloadedAt = in.Metadata.LastAccountDownload
	}

	return user
}
