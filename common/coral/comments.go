//go:generate easyjson -all comments.go
package coral

import "time"

type RevisionPerspective struct {
	Score float64 `json:"score"`
	Model string  `json:"model,intern"`
}

// RevisionMetadata is the metadata associated with a given Revision for a
// Comment in Coral.
type RevisionMetadata struct {
	Akismet     *bool                `json:"akismet,omitempty"`
	Perspective *RevisionPerspective `json:"perspective,omitempty"`
}

// Revision is a given revision of a Comment in Coral.
type Revision struct {
	ID           string           `json:"id" conform:"trim" validate:"required"`
	Body         HTML             `json:"body" conform:"trim" validate:"required"`
	ActionCounts map[string]int   `json:"actionCounts" validate:"required"`
	Metadata     RevisionMetadata `json:"metadata" validate:"required"`
	CreatedAt    Time             `json:"createdAt" validate:"required"`
}

// CommentTag is a Tag associated with a Comment in Coral.
type CommentTag struct {
	Type      string `json:"type,intern" conform:"trim" validate:"oneof=STAFF FEATURED REVIEW QUESTION,required"`
	CreatedBy string `json:"createdBy,omitempty"`
	CreatedAt Time   `json:"createdAt" validate:"required"`
}

// Comment is the base Coral Comment that is used in Coral.
type Comment struct {
	TenantID         string                 `json:"tenantID,intern" validate:"required"`
	ID               string                 `json:"id" conform:"trim" validate:"required"`
	SiteID           string                 `json:"siteID,intern" validate:"required"`
	AncestorIDs      []string               `json:"ancestorIDs" validate:"required"`
	ParentID         string                 `json:"parentID,omitempty" conform:"trim"`
	ParentRevisionID string                 `json:"parentRevisionID,omitempty" conform:"trim"`
	AuthorID         string                 `json:"authorID" conform:"trim" validate:"required"`
	StoryID          string                 `json:"storyID" conform:"trim" validate:"required"`
	Revisions        []Revision             `json:"revisions" validate:"required"`
	Status           string                 `json:"status,intern" conform:"trim" validate:"oneof=NONE APPROVED REJECTED PREMOD SYSTEM_WITHHELD,required"`
	ActionCounts     map[string]int         `json:"actionCounts" validate:"required"`
	ChildIDs         []string               `json:"childIDs" validate:"required"`
	Tags             []CommentTag           `json:"tags" validate:"required"`
	ChildCount       int                    `json:"childCount" validate:"gte=0"`
	CreatedAt        Time                   `json:"createdAt" validate:"required"`
	DeletedAt        *Time                  `json:"deletedAt,omitempty"`
	ImportedAt       Time                   `json:"importedAt"`
	Rating           int                    `json:"rating,omitempty"`
	Extra            map[string]interface{} `json:"extra"`
}

// NewComment will return an initialized Comment.
func NewComment(tenantID, siteID string) *Comment {
	return &Comment{
		TenantID:     tenantID,
		SiteID:       siteID,
		AncestorIDs:  []string{},
		Revisions:    []Revision{},
		ActionCounts: map[string]int{},
		ChildIDs:     []string{},
		Tags:         []CommentTag{},
		ImportedAt:   Time{Time: time.Now()},
		Extra:        map[string]interface{}{},
	}
}
