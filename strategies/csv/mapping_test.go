package csv_test

import (
	"coral-importer/strategies/csv"
	"regexp"
	"testing"
)

func TestParseUser(t *testing.T) {
	upper, err := regexp.Compile("[A-Z]")
	if err != nil {
		t.Fatalf("could not create RegExp for testing email addresses, %v", err)
	}

	tests := [][]string{
		{"id", "Email@addRess.com", "username", "", "", ""},
		{"id", "email@address.com", "username", "", "", ""},
	}
	for i, test := range tests {
		user, err := csv.ParseUser(test)
		if err != nil {
			t.Errorf("[%d] expected no error, got %v", i, err)
		}

		if upper.MatchString(user.Email) {
			t.Fatalf("[%d] found uppercase characters in email address, %s", i, user.Email)
		}
	}
}
