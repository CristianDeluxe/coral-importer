package legacy

import (
	"path/filepath"

	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/strategies"
)

type CommentRef struct {
	Status       string
	StoryID      string
	ParentID     string
	ActionCounts map[string]int
}

type StoryRef struct {
	ActionCounts map[string]int
	StatusCounts coral.CommentStatusCounts
	Flagged      int
}

type UserRef struct {
	StatusCounts coral.CommentStatusCounts
}

func NewContext(c strategies.Context) *Context {
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

	return &Context{
		TenantID: tenantID,
		SiteID:   siteID,
		Filenames: Filenames{
			Input: InputFilenames{
				Comments: filepath.Join(input, "comments.json"),
				Actions:  filepath.Join(input, "actions.json"),
				Assets:   filepath.Join(input, "assets.json"),
				Users:    filepath.Join(input, "users.json"),
			},
			Output: OutputFilenames{
				Comments:       filepath.Join(output, "comments.json"),
				CommentActions: filepath.Join(output, "commentActions.json"),
				Stories:        filepath.Join(output, "stories.json"),
				Users:          filepath.Join(output, "users.json"),
			},
		},
		Reconstructor: common.NewReconstructor(),
		users:         map[string]*UserRef{},
		stories:       map[string]*StoryRef{},
		comments:      map[string]*CommentRef{},
	}
}

type InputFilenames struct {
	Comments string
	Actions  string
	Assets   string
	Users    string
}

type OutputFilenames struct {
	Comments       string
	CommentActions string
	Stories        string
	Users          string
}

type Filenames struct {
	Input  InputFilenames
	Output OutputFilenames
}

type Context struct {
	TenantID      string
	SiteID        string
	Filenames     Filenames
	Reconstructor *common.Reconstructor

	users    map[string]*UserRef
	stories  map[string]*StoryRef
	comments map[string]*CommentRef
}

func (ctx *Context) ReleaseUsers() {
	ctx.users = nil
}

func (ctx *Context) FindOrCreateUser(id string) (*UserRef, bool) {
	ref, ok := ctx.users[id]
	if !ok {
		ref = &UserRef{}
		ctx.users[id] = ref
	}

	return ref, ok
}

func (ctx *Context) FindUser(id string) (*UserRef, bool) {
	ref, ok := ctx.users[id]
	return ref, ok
}

func (ctx *Context) ReleaseStories() {
	ctx.stories = nil
}

func (ctx *Context) FindOrCreateStory(id string) (*StoryRef, bool) {
	ref, ok := ctx.stories[id]
	if !ok {
		ref = &StoryRef{
			ActionCounts: map[string]int{},
		}
		ctx.stories[id] = ref
	}

	return ref, ok
}

func (ctx *Context) FindStory(id string) (*StoryRef, bool) {
	ref, ok := ctx.stories[id]
	return ref, ok
}

func (ctx *Context) ReleaseComments() {
	ctx.comments = nil
}

func (ctx *Context) FindOrCreateComment(id string) (*CommentRef, bool) {
	ref, ok := ctx.comments[id]
	if !ok {
		ref = &CommentRef{
			ActionCounts: map[string]int{},
		}
		ctx.comments[id] = ref
	}

	return ref, ok
}

func (ctx *Context) FindComment(id string) (*CommentRef, bool) {
	ref, ok := ctx.comments[id]
	return ref, ok
}
