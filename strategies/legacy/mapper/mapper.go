package mapper

import (
	"encoding/json"
	"os"

	"github.com/coralproject/coral-importer/common/coral"
	"github.com/coralproject/coral-importer/internal/utility"
	"github.com/coralproject/coral-importer/internal/warnings"
	"github.com/mailru/easyjson"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type User map[string]interface{}

// Update describes an update for a User.
type Update struct {
	// Username is the new username that should be associated with the user.
	Username string

	// Email is the new email that should be associated with the user.
	Email string

	// SSO is the ID of the SSO user profile that should be associated with the
	// user.
	SSO string
}

type Operation func(pre *User, update *Update) bool

func New(dryRun bool) *Mapper {
	return &Mapper{
		dryRun:  dryRun,
		updates: make(map[string]Update),
	}
}

type Mapper struct {
	dryRun     bool
	operations []Operation
	updates    map[string]Update
}

// LoadConfig will load the configuration from the specified file.
func (m *Mapper) LoadConfig(configFilename string) error {
	f, err := os.Open(configFilename)
	if err != nil {
		return errors.Wrap(err, "could not open --config file")
	}
	defer f.Close()

	var config struct {
		Users struct {
			Username string `json:"username"`
			SSO      struct {
				ID       string `json:"id"`
				Provider string `json:"provider"`
				Email    string `json:"email"`
			} `json:"sso"`
		} `json:"users"`
	}

	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return errors.Wrap(err, "could not load config file")
	}

	if config.Users.Username != "" {
		getter := newGetter(config.Users.Username)
		m.operations = append(m.operations, func(pre *User, update *Update) bool {
			// Try to get the username from the pre user.
			username, ok := getter.Get(*pre)
			if !ok {
				return false
			}

			update.Username = username
			return true
		})
	}

	if config.Users.SSO.ID != "" {
		getter := newGetter(config.Users.SSO.ID)
		m.operations = append(m.operations, func(pre *User, update *Update) bool {
			// Try to get the SSO ID from the pre user.
			id, ok := getter.Get(*pre)
			if !ok {
				return false
			}

			update.SSO = id

			return true
		})
	} else if config.Users.SSO.Provider != "" {
		var getter *getter

		// If the SSO Email is enabled, setup the getter for it.
		if config.Users.SSO.Email != "" {
			getter = newGetter(config.Users.SSO.Email)
		}

		m.operations = append(m.operations, func(pre *User, update *Update) bool {
			// Try to get the profiles from the user.
			profiles, ok := (*pre)["profiles"].([]interface{})
			if !ok {
				return false
			}

			// Iterate over the profiles to find the profile with the specified
			// provider.
			for _, profile := range profiles {
				p, ok := profile.(map[string]interface{})
				if !ok {
					return false
				}

				provider, ok := p["provider"].(string)
				if !ok {
					return false
				}

				if provider != config.Users.SSO.Provider {
					continue
				}

				// Looks like we found the provider! Get the ID from the provider.
				id, ok := p["id"].(string)
				if !ok {
					return false
				}

				update.SSO = id

				// If we have the email getter enabled... Then also check for that!
				if getter == nil {
					return true
				}

				email, ok := getter.Get(p)
				if !ok {
					return true
				}

				update.Email = email

				return true
			}

			return false
		})
	} else if config.Users.SSO.Email != "" {
		return errors.New("must specify users.sso.provider when specifying users.sso.email")
	}

	return nil
}

func (m *Mapper) Pre(input string) error {
	bar, err := utility.NewLineCounter("(1/2) Computing User Updates", input)
	if err != nil {
		return errors.Wrap(err, "could not count input users file")
	}
	defer bar.Finish()

	return utility.ReadJSON(input, func(line int, data []byte) error {
		defer bar.Increment()

		var user User
		if err := json.Unmarshal(data, &user); err != nil {
			return errors.Wrap(err, "could not load a user in the --pre file")
		}

		id, ok := user["id"].(string)
		if !ok {
			return errors.New("could not get the user id from a user in --pre file")
		}

		var updated bool

		var update Update
		for _, operation := range m.operations {
			updated = operation(&user, &update) || updated
		}

		if !updated {
			return nil
		}

		m.updates[id] = update

		return nil
	})
}

func (m *Mapper) Post(output, post string) error {
	writer, err := utility.NewJSONWriter(m.dryRun, post)
	if err != nil {
		return errors.Wrap(err, "could not create writer")
	}
	defer writer.Close()

	bar, err := utility.NewLineCounter("(2/2) Writing User Updates", output)
	if err != nil {
		return errors.Wrap(err, "could not count output users file")
	}
	defer bar.Finish()

	return utility.ReadJSON(output, func(line int, data []byte) error {
		defer bar.Increment()

		var user coral.User
		if err := easyjson.Unmarshal(data, &user); err != nil {
			return errors.Wrap(err, "could not load a user")
		}

		// Check to see if we have updates for this user.
		update, ok := m.updates[user.ID]
		if !ok {
			if err := writer.Write(user); err != nil {
				return errors.Wrap(err, "could not write user")
			}

			return nil
		}

		if update.SSO != "" {
			// Check that the user's ID matches the SSO ID.
			if update.SSO != user.ID {
				warnings.SSOIDMismatch.Once(func() {
					logrus.WithField("id", user.ID).Warn("found a user that had an SSO ID that did not match their User ID")
				})
			}

			// Add the new profile.
			user.Profiles = append(user.Profiles, coral.UserProfile{
				ID:   update.SSO,
				Type: "sso",
			})
		}

		if update.Email != "" {
			user.Email = update.Email
		}

		if update.Username != "" {
			user.Username = update.Username
		}

		// Remove the update.
		delete(m.updates, user.ID)

		if err := writer.Write(user); err != nil {
			return errors.Wrap(err, "could not write user")
		}

		return nil
	})
}
