package common

import "github.com/coralproject/coral-importer/common/coral"

type Reconstructor struct {
	parents  map[string]string
	children map[string][]string
}

func (r *Reconstructor) AddComment(comment *coral.Comment) {
	r.AddIDs(comment.ID, comment.ParentID)
}

func (r *Reconstructor) AddIDs(id, parentID string) {
	// Save the comment's parent.
	r.parents[id] = parentID

	// Add the comment to the array of children, if it has any.
	if parentID != "" {
		if _, ok := r.children[parentID]; !ok {
			r.children[parentID] = []string{id}
		} else {
			r.children[parentID] = append(r.children[parentID], id)
		}
	}
}

// GetChildren will get the linked children for a Comment.
func (r *Reconstructor) GetChildren(commentID string) []string {
	children, ok := r.children[commentID]
	if ok {
		return children
	}

	return []string{}
}

// GetParent will get a parent (if it exists) for a Comment.
func (r *Reconstructor) GetParent(commentID string) string {
	parentID, ok := r.parents[commentID]
	if ok && parentID != "" {
		return parentID
	}

	return ""
}

// GetAncestors will get the array of ancestors for a given Comment.
func (r *Reconstructor) GetAncestors(commentID string) []string {
	// Store the ancestors in an array.
	ancestorIDs := []string{}

	parentID := r.GetParent(commentID)
	for parentID != "" {
		// Add the ID of this comment to the ancestorIDs.
		ancestorIDs = append(ancestorIDs, parentID)

		// Store the reference to the ancestorParentID
		ancestorParentID, ok := r.parents[parentID]
		if !ok {
			return ancestorIDs
		}

		parentID = ancestorParentID
	}

	return ancestorIDs
}

func NewReconstructor() *Reconstructor {
	return &Reconstructor{
		parents:  make(map[string]string),
		children: make(map[string][]string),
	}
}
