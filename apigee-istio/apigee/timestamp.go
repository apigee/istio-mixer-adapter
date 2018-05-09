package apigee

import (
	"fmt"
	"strconv"
	"time"
)

// Timestamp represents a time that can be unmarshalled from a JSON string
// formatted as "java time" = milliseconds-since-unix-epoch.
type Timestamp struct {
	time.Time
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	ms := t.Time.UnixNano() / 1000000
	stamp := fmt.Sprintf("%d", ms)
	return []byte(stamp), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Time is expected in RFC3339 or Unix format.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	ms, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return err
	}
	t.Time = time.Unix(int64(ms/1000), (ms-int64(ms/1000)*1000)*1000000)
	return nil
}

func (t Timestamp) String() string {
	return fmt.Sprintf("%d", int64(t.Time.UnixNano())/1000000)
}

// Equal reports whether t and u are equal based on time.Equal
func (t Timestamp) Equal(u Timestamp) bool {
	return t.Time.Equal(u.Time)
}
