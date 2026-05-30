package types

import "testing"

func TestIsValidTableStyleID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "engine default GUID", id: DefaultTableStyleID, want: true},
		{name: "uppercase GUID", id: "{ABCDEF01-1234-5678-9ABC-DEF012345678}", want: true},
		{name: "lowercase GUID", id: "{abcdef01-1234-5678-9abc-def012345678}", want: true},
		{name: "empty", id: "", want: false},
		{name: "sentinel is not a GUID", id: "@template-default", want: false},
		{name: "missing braces", id: "5C22544A-7EE6-4342-B048-85BDC9FD1C3A", want: false},
		{name: "non-hex characters", id: "{ZZZZZZZZ-1234-5678-9ABC-DEF012345678}", want: false},
		{name: "wrong group lengths", id: "{ABC-1234-5678-9ABC-DEF012345678}", want: false},
		{name: "xml attribute injection", id: `{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"/><a:evil/>`, want: false},
		{name: "raw metacharacters", id: `bad"&<>`, want: false},
		{name: "trailing junk after GUID", id: "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}extra", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTableStyleID(tt.id); got != tt.want {
				t.Errorf("IsValidTableStyleID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
