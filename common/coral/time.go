package coral

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Time struct {
	time.Time
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"$date": t.Time,
	})
}

func (t *Time) UnmarshalJSON(data []byte) error {
	var tmp interface{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return errors.Wrap(err, "could not parse time JSON")
	}

	switch v := tmp.(type) {
	case float64:
		// It's in a milliseconds representation, but in exponential format.
		// Ex: 1524510261939e+12
		t.Time = time.Unix(int64(v)/1000, 0)
	case map[string]interface{}:
		date, ok := v["$date"].(string)
		if !ok || date == "" {
			if obj, ok := v["$date"].(map[string]interface{}); ok {
				// Try to handle the case where we get something that looks like
				// this: {"$date":{"$numberLong":"-62075098782000"}}
				if _, ok := obj["$numberLong"].(string); ok {
					logrus.Warn("saw a date in the format: { $date: { $numberLong: \"-62075098782000\" } }")

					return nil
				}

				// Try to handle the case where we get something that looks like
				// this: {"$date":{"$numberLong":-62075098782000"}
				if _, ok := obj["$numberLong"].(float64); ok {
					logrus.Warn("saw a date in the format: { $date: { $numberLong: \"-62075098782000\" } }")

					return nil
				}
			}

			return errors.Errorf("invalid format: %#v, %s", v["$date"], reflect.TypeOf(v["$date"]))
		}

		tt, err := time.Parse(time.RFC3339, date)
		if err != nil {
			return errors.Wrap(err, "could not parse $date format")
		}

		t.Time = tt
	case string:
		tt, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return errors.Wrap(err, "could not parse $date format")
		}

		t.Time = tt
	default:
		return errors.Errorf("unsupported time format: %v %T", string(data), tmp)
	}

	return nil
}
