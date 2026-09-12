// Package hookio parses the stdin JSON Claude Code passes to SessionStart
// and PostModelSwitch hooks.
package hookio

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Input is the subset of Claude Code's hook stdin JSON keyway reads.
// SessionStart carries Model — can be omitted after /clear or a
// conversation-recovery restore, per the docs — but only fires once, at
// session start. PostModelSwitch carries ToModel instead, on every model
// change afterward: manual /model, fast-mode toggling, automatic
// fallback, opusplan transitions, and the model restored on resume.
type Input struct {
	Cwd           string `json:"cwd"`
	HookEventName string `json:"hook_event_name"`
	Model         string `json:"model"`
	ToModel       string `json:"to_model"`
}

// ActiveModel returns whichever of ToModel/Model is set, preferring
// ToModel: when both are present (they aren't, in practice) it's the more
// specific value.
func (in Input) ActiveModel() string {
	if in.ToModel != "" {
		return in.ToModel
	}
	return in.Model
}

// Parse decodes stdin JSON. An empty or unparseable body is not an error —
// hooks must fail soft, so callers get a zero-valued Input and proceed.
func Parse(stdin io.Reader) Input {
	var in Input
	data, err := io.ReadAll(stdin)
	if err != nil || len(data) == 0 {
		return in
	}
	_ = json.Unmarshal(data, &in)
	return in
}

// Disabled reports whether KEYWAY_DISABLE is set to anything but an
// explicit off value, so an eval harness that starts fresh sessions can
// suppress a globally-registered SessionStart hook from injecting tier
// content into a controlled probe.
func Disabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KEYWAY_DISABLE"))) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}
