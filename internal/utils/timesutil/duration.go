// wrapper that embeds time.Duration to make the json package able to marshal a string representation of duration
// to an actual object of type time.Duration
// (for example the json key:value "duration": "5s", would be parsed to a duration of time.Second * 5)
package timesutil

import (
	"encoding/json"
	"fmt"
	"time"
)

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	parsed, err := time.ParseDuration(s)
	if err == nil {
		*d = Duration(parsed)
		return nil
	}

	return fmt.Errorf("failed to parse duration from JSON: %s", string(b))
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

func FromDuration(duration time.Duration) Duration {
	return Duration(duration)
}

func FromString(s string) (Duration, error) {
	dur, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("could not parse duration: %w", err)
	}

	return Duration(dur), nil
}
