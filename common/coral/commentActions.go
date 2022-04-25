//go:generate easyjson -all commentActions.go
package coral

import "time"

// CommentAction is the base Coral Comment Action that represents an action
// against a Comment.
type CommentAction struct {
	TenantID          string                 `json:"tenantID,intern" validate:"required"`
	ID                string                 `json:"id" conform:"trim" validate:"required"`
	SiteID            string                 `json:"siteID,intern" validate:"required"`
	ActionType        string                 `json:"actionType,intern" validate:"oneof=REACTION DONT_AGREE FLAG,required"`
	CommentID         string                 `json:"commentID" validate:"required"`
	CommentRevisionID string                 `json:"commentRevisionID" validate:"required"`
	Reason            string                 `json:"reason,omitempty,intern" validate:"omitempty,oneof= COMMENT_DETECTED_BANNED_WORD COMMENT_DETECTED_LINKS COMMENT_DETECTED_PREMOD_USER COMMENT_DETECTED_RECENT_HISTORY COMMENT_DETECTED_REPEAT_POST COMMENT_DETECTED_SPAM COMMENT_DETECTED_SUSPECT_WORD COMMENT_DETECTED_TOXIC COMMENT_REPORTED_OFFENSIVE COMMENT_REPORTED_OTHER COMMENT_REPORTED_SPAM"`
	AdditionalDetails string                 `json:"additionalDetails,omitempty"`
	StoryID           string                 `json:"storyID" validate:"required"`
	UserID            *string                `json:"userID" validate:"required"`
	CreatedAt         Time                   `json:"createdAt"`
	Metadata          map[string]interface{} `json:"metadata"`
	ImportedAt        Time                   `json:"importedAt"`
	Extra             map[string]interface{} `json:"extra"`
}

// NewCommentAction will return an initialized CommentAction.
func NewCommentAction(tenantID, siteID string) *CommentAction {
	return &CommentAction{
		TenantID:   tenantID,
		SiteID:     siteID,
		Metadata:   map[string]interface{}{},
		ImportedAt: Time{Time: time.Now()},
		CreatedAt:  NewCursorTime(),
	}
}
