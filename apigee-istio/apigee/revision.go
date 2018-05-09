package apigee

import (
	"fmt"
	"strconv"
	"strings"
)

// Revision represents a revision number. Edge returns rev numbers in string form.
// This marshals and unmarshals between that format and int.
type Revision int

// MarshalJSON implements the json.Marshaler interface. It marshals from
// a Revision holding an integer value like 2, into a string like "2".
func (r *Revision) MarshalJSON() ([]byte, error) {
	rev := fmt.Sprintf("%d", r)
	return []byte(rev), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface. It unmarshals from
// a string like "2" (including the quotes), into an integer 2.
func (r *Revision) UnmarshalJSON(b []byte) error {
	rev, e := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(string(b), "\""), "\""), 10, 32)
	if e != nil {
		return e
	}

	*r = Revision(rev)
	return nil
}

func (r Revision) String() string {
	return fmt.Sprintf("%d", r)
}
