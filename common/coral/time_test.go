package coral_test

import (
	"coral-importer/common/coral"
	"testing"
)

func TestTimeUnmarshalJSON(t *testing.T) {
	var tm coral.Time

	inputs := []struct {
		name  string
		value string
	}{
		{name: "normal date time", value: `{ "$date": "2018-10-25T23:57:49.053Z" }`},
		{name: "negative number long", value: `{ "$date": { "$numberLong": -62075098782000 } }`},
		{name: "negative number string long", value: `{ "$date": { "$numberLong": "-62075098782000" } }`},
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			if err := tm.UnmarshalJSON([]byte(input.value)); err != nil {
				t.Errorf("expected no error with input %s, got %v", input.value, err)
			}
		})
	}
}
