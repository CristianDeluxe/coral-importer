//go:generate easyjson -all users.go
package coral

import "time"

type UserProfile struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Password     string `json:"password,omitempty"`
	PasswordID   string `json:"passwordID,omitempty"`
	LastIssuedAt *Time  `json:"lastIssuedAt,omitempty"`
}

type UserNotifications struct {
	OnReply         bool   `json:"onReply"`
	OnFeatured      bool   `json:"onFeatured"`
	OnStaffReplies  bool   `json:"onStaffReplies"`
	OnModeration    bool   `json:"onModeration"`
	DigestFrequency string `json:"digestFrequency" validate:"oneof=NONE DAILY HOURLY"`
}

func NewUserNotifications() UserNotifications {
	return UserNotifications{
		DigestFrequency: "NONE",
	}
}

type TimeRange struct {
	Start  Time `json:"start"`
	Finish Time `json:"finish"`
}

type UserSuspensionStatusHistory struct {
	ID         string    `json:"id"`
	From       TimeRange `json:"from"`
	CreatedBy  string    `json:"createdBy,omitempty"`
	CreatedAt  Time      `json:"createdAt"`
	ModifiedBy *string   `json:"modifiedBy,omitempty"`
	ModifiedAt *Time     `json:"modifiedAt,omitempty"`
	Message    string    `json:"message"`
}

type UserSuspensionStatus struct {
	History []UserSuspensionStatusHistory `json:"history"`
}

type UserBanStatusHistory struct {
	ID        string `json:"id"`
	Active    bool   `json:"active"`
	CreatedBy string `json:"createdBy,omitempty"`
	CreatedAt Time   `json:"createdAt"`
	Message   string `json:"message,omitempty"`
}

type UserBanStatus struct {
	Active  bool                   `json:"active"`
	History []UserBanStatusHistory `json:"history"`
}

type UserUsernameStatusHistory struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedBy string `json:"createdBy"`
	CreatedAt Time   `json:"createdAt"`
}

type UserUsernameStatus struct {
	History []UserUsernameStatusHistory `json:"history"`
}

func NewUserUsernameStatus() UserUsernameStatus {
	return UserUsernameStatus{
		History: []UserUsernameStatusHistory{},
	}
}

type UserPremodStatusHistory struct {
	AssignedBy string `json:"assignedBy"`
	Status     bool   `json:"status"`
	CreatedAt  Time   `json:"createdAt"`
}

type UserPremodStatus struct {
	Active  bool                      `json:"active"`
	History []UserPremodStatusHistory `json:"history"`
}

type UserStatus struct {
	SuspensionStatus UserSuspensionStatus `json:"suspension"`
	BanStatus        UserBanStatus        `json:"ban"`
	UsernameStatus   UserUsernameStatus   `json:"username"`
	PremodStatus     UserPremodStatus     `json:"premod"`
}

func NewUserStatus() UserStatus {
	return UserStatus{
		SuspensionStatus: UserSuspensionStatus{
			History: []UserSuspensionStatusHistory{},
		},
		BanStatus: UserBanStatus{
			History: []UserBanStatusHistory{},
		},
		UsernameStatus: NewUserUsernameStatus(),
		PremodStatus: UserPremodStatus{
			History: []UserPremodStatusHistory{},
		},
	}
}

type IgnoredUser struct {
	ID        string `json:"id"`
	CreatedAt Time   `json:"createdAt"`
}

type UserToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt Time   `json:"createdAt"`
}

type UserCommentCounts struct {
	Status CommentStatusCounts `json:"status"`
}

type User struct {
	TenantID         string                 `json:"tenantID" validate:"required"`
	ID               string                 `json:"id" conform:"trim" validate:"required"`
	Username         string                 `json:"username" validate:"required"`
	Email            string                 `json:"email,omitempty" conform:"email,lower" validate:"email"`
	Profiles         []UserProfile          `json:"profiles,omitempty"`
	Role             string                 `json:"role" validate:"required,oneof=COMMENTER STAFF MODERATOR ADMIN"`
	Notifications    UserNotifications      `json:"notifications"`
	ModeratorNotes   []string               `json:"moderatorNotes"`
	Status           UserStatus             `json:"status"`
	CreatedAt        Time                   `json:"createdAt" validate:"required"`
	IgnoredUsers     []IgnoredUser          `json:"ignoredUsers"`
	Tokens           []UserToken            `json:"tokens"`
	CommentCounts    UserCommentCounts      `json:"commentCounts"`
	LastDownloadedAt *Time                  `json:"lastDownloadedAt"`
	ImportedAt       Time                   `json:"importedAt"`
	Extra            map[string]interface{} `json:"extra"`
}

func NewUser(tenantID string) *User {
	return &User{
		TenantID:       tenantID,
		Notifications:  NewUserNotifications(),
		Status:         NewUserStatus(),
		ModeratorNotes: []string{},
		Tokens:         []UserToken{},
		Role:           "COMMENTER",
		ImportedAt:     Time{Time: time.Now()},
		CreatedAt:      NewCursorTime(),
	}
}
