package coral_test

import (
	"encoding/json"
	"testing"

	"github.com/coralproject/coral-importer/common"
	"github.com/coralproject/coral-importer/common/coral"
)

func TestComment(t *testing.T) {
	input := `
	{
		"id": "5bfb82e6-b53c-4e2c-824e-e12d53470543",
		"siteID": "2c193df7-2cd7-4653-8d0e-d69514e8f220",
		"tenantID": "c2440817-464e-4a8f-8851-24effd8fee9d",
		"childIDs": [],
		"childCount": 0,
		"revisions": [
		  {
			"id": "73248610-9137-47ef-b135-46dbe8fb25fb",
			"body": "First comment.<br>",
			"actionCounts": {
			  "REACTION": 1
			},
			"metadata": {
			  "linkCount": 0
			},
			"createdAt": "2019-09-25T19:26:03.034Z"
		  }
		],
		"createdAt": "2019-09-25T19:26:03.034Z",
		"authorID": "124d0888-4016-4630-8871-40ee20c9804a",
		"storyID": "05e57db4-cf6c-4e0c-a672-5eed3c8a47f7",
		"tags": [
		  {
			"type": "STAFF",
			"createdAt": "2019-09-25T19:26:03.034Z"
		  }
		],
		"status": "APPROVED",
		"ancestorIDs": [],
		"actionCounts": {
		  "REACTION": 1
		}
	  }
	`

	// The comment that we're loading the sample JSON into.
	var comment coral.Comment

	// Unmarshal the comment.
	if err := json.Unmarshal([]byte(input), &comment); err != nil {
		t.Fatal(err)
	}

	// Validate the comment.
	if err := common.Validate(comment); err != nil {
		t.Fatal(err)
	}
}
