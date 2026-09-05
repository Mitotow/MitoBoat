package commands

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    Invocation
		wantOK  bool
	}{
		{name: "empty", message: "", wantOK: false},
		{name: "plain text", message: "test", wantOK: false},
		{name: "prefix only", message: "!", wantOK: false},
		{name: "prefix and spaces", message: "!   ", wantOK: false},
		{name: "prefix not first", message: "hello !test", wantOK: false},
		{
			name:    "single letter",
			message: "!t",
			want:    Invocation{Name: "t", Args: []string{}},
			wantOK:  true,
		},
		{
			name:    "no args",
			message: "!test",
			want:    Invocation{Name: "test", Args: []string{}},
			wantOK:  true,
		},
		{
			name:    "surrounding whitespace is ignored",
			message: "  !test  ",
			want:    Invocation{Name: "test", Args: []string{}},
			wantOK:  true,
		},
		{
			name:    "name is lowercased",
			message: "!TeSt",
			want:    Invocation{Name: "test", Args: []string{}},
			wantOK:  true,
		},
		{
			name:    "args are captured",
			message: "!test one two",
			want:    Invocation{Name: "test", Args: []string{"one", "two"}},
			wantOK:  true,
		},
		{
			name:    "args keep their case and collapse whitespace",
			message: "!test   One    Two",
			want:    Invocation{Name: "test", Args: []string{"One", "Two"}},
			wantOK:  true,
		},
		{
			name:    "space between prefix and name",
			message: "! test",
			want:    Invocation{Name: "test", Args: []string{}},
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Parse(tt.message)
			if ok != tt.wantOK {
				t.Fatalf("Parse(%q) ok = %v, want %v", tt.message, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Name != tt.want.Name {
				t.Errorf("Parse(%q) name = %q, want %q", tt.message, got.Name, tt.want.Name)
			}
			if !reflect.DeepEqual(got.Args, tt.want.Args) {
				t.Errorf("Parse(%q) args = %#v, want %#v", tt.message, got.Args, tt.want.Args)
			}
		})
	}
}

// Parse runs on every chat message the bot sees, so a regression that starts
// allocating here is worth catching.
func BenchmarkParseNonCommand(b *testing.B) {
	for b.Loop() {
		Parse("just a normal chat message")
	}
}

func BenchmarkParseCommand(b *testing.B) {
	for b.Loop() {
		Parse("!command with some args")
	}
}
