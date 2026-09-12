package hookio

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Input
	}{
		{
			name:  "SessionStart payload",
			input: `{"cwd":"/home/x/proj","hook_event_name":"SessionStart","model":"claude-sonnet-5"}`,
			want:  Input{Cwd: "/home/x/proj", HookEventName: "SessionStart", Model: "claude-sonnet-5"},
		},
		{
			name:  "PostModelSwitch payload",
			input: `{"cwd":"/home/x/proj","hook_event_name":"PostModelSwitch","from_model":"claude-sonnet-5","to_model":"claude-opus-5"}`,
			want:  Input{Cwd: "/home/x/proj", HookEventName: "PostModelSwitch", ToModel: "claude-opus-5"},
		},
		{
			name:  "empty stdin",
			input: "",
			want:  Input{},
		},
		{
			name:  "malformed JSON fails soft",
			input: "{not json",
			want:  Input{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(strings.NewReader(tt.input))
			if got != tt.want {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestActiveModel(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want string
	}{
		{"prefers ToModel", Input{Model: "claude-sonnet-5", ToModel: "claude-opus-5"}, "claude-opus-5"},
		{"falls back to Model", Input{Model: "claude-sonnet-5"}, "claude-sonnet-5"},
		{"both empty", Input{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.ActiveModel(); got != tt.want {
				t.Errorf("ActiveModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisabled(t *testing.T) {
	t.Setenv("KEYWAY_DISABLE", "")
	if Disabled() {
		t.Error("Disabled() = true for empty env var")
	}
	t.Setenv("KEYWAY_DISABLE", "1")
	if !Disabled() {
		t.Error("Disabled() = false for KEYWAY_DISABLE=1")
	}
	t.Setenv("KEYWAY_DISABLE", "off")
	if Disabled() {
		t.Error("Disabled() = true for KEYWAY_DISABLE=off")
	}
}
