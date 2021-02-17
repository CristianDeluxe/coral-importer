package csv

import (
	"strconv"
	"strings"

	"coral-importer/common"

	"github.com/pkg/errors"
)

// TranslateCommentStatus will convert the status that is expected as a part of
// the CSV import to the correct Coral status implementing a safe fallback.
func TranslateCommentStatus(status string) string {
	switch strings.ToUpper(status) {
	case "APPROVED":
		return "APPROVED"
	case "REJECTED":
		return "REJECTED"
	case "NONE":
		fallthrough
	default:
		return "NONE"
	}
}

// CommentColumns is the number of expected columns in the comments.csv file.
const CommentColumns = 8

// Comment is the string representation of a coral.Comment as it is imported in
// the CSV format.
type Comment struct {
	ID        string `conform:"trim" validate:"required"`
	AuthorID  string `conform:"trim" validate:"required"`
	StoryID   string `conform:"trim" validate:"required"`
	CreatedAt string `conform:"trim" validate:"required"`
	Body      string `conform:"trim" validate:"required_without=Rating"`
	ParentID  string `conform:"trim"`
	Status    string `conform:"trim"`
	Rating    int
}

// ParseComment will extract a Comment from the fields and perform validation on
// the input.
func ParseComment(fields []string) (*Comment, error) {
	comment := Comment{
		ID:        fields[0],
		AuthorID:  fields[1],
		StoryID:   fields[2],
		CreatedAt: fields[3],
		Body:      fields[4],
		ParentID:  fields[5],
		Status:    TranslateCommentStatus(fields[6]),
	}

	if fields[7] != "" {
		rating, err := strconv.ParseInt(fields[7], 10, 32)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert to int")
		}

		comment.Rating = int(rating)
	}

	if err := common.Check(&comment); err != nil {
		return nil, errors.Wrap(err, "could not validate comment")
	}

	return &comment, nil
}

// StoryColumns is the number of expected columns in the stories.csv file.
const StoryColumns = 7

// Story is the string representation of a coral.Story as it is imported in the
// CSV format.
type Story struct {
	ID          string `conform:"trim" validate:"required"`
	URL         string `conform:"trim" validate:"required,url"`
	Title       string `conform:"trim"`
	Author      string `conform:"trim"`
	PublishedAt string `conform:"trim"`
	ClosedAt    string `conform:"trim"`
	Mode        string `conform:"trim,upper" validate:"omitempty,oneof= COMMENTS QA RATINGS_AND_REVIEWS"`
}

// ParseStory will extract a Story from the fields and perform validation on the
// input.
func ParseStory(fields []string) (*Story, error) {
	story := Story{
		ID:          fields[0],
		URL:         fields[1],
		Title:       fields[2],
		Author:      fields[3],
		PublishedAt: fields[4],
		ClosedAt:    fields[5],
		Mode:        fields[6],
	}

	if err := common.Check(&story); err != nil {
		return nil, errors.Wrap(err, "could not validate story")
	}

	return &story, nil
}

// TranslateUserRole will ensure the role value is a valid Coral role.
func TranslateUserRole(role string) string {
	switch strings.ToUpper(role) {
	case "ADMIN":
		return "ADMIN"
	case "MODERATOR":
		return "MODERATOR"
	case "COMMENTER":
		return "COMMENTER"
	default:
		return "COMMENTER"
	}
}

// UserColumns is the number of expected columns in the users.csv file.
const UserColumns = 6

// User is the string representation of a coral.User as it is imported in the
// CSV format.
type User struct {
	ID        string `conform:"trim" validate:"required"`
	Email     string `conform:"trim" validate:"email"`
	Username  string `conform:"trim" validate:"required"`
	Role      string `conform:"trim"`
	Banned    string `conform:"trim"`
	CreatedAt string `conform:"trim"`
}

// ParseUser will extract a User from the fields and perform validation on the
// input.
func ParseUser(fields []string) (*User, error) {
	user := User{
		ID:        fields[0],
		Email:     fields[1],
		Username:  fields[2],
		Role:      fields[3],
		Banned:    fields[4],
		CreatedAt: fields[5],
	}

	if err := common.Check(&user); err != nil {
		return nil, errors.Wrap(err, "could not validate user")
	}

	return &user, nil
}
