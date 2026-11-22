package alarms

import (
	"fmt"
	"strconv"
	"strings"
)

// ConditionScope is what an activity-based alarm's threshold is measured
// against.
type ConditionScope string

const (
	ScopeCategory ConditionScope = "category"
	ScopeProject  ConditionScope = "project"
)

// Condition is a parsed alarms.trigger_condition - spec section 22's own
// example is `"category=Gaming;minutes=60"` ("After 1 hour of gaming"),
// which is exactly this grammar: semicolon-separated key=value pairs,
// scope name (category's or project's NAME, resolved to an ID by
// internal/scheduler at evaluation time, not stored here) plus a minute
// threshold.
type Condition struct {
	Scope   ConditionScope
	Name    string
	Minutes int
}

// ParseTriggerCondition parses the format documented on Condition.
// Rejects anything malformed rather than guessing, same reasoning as
// ParseRecurrenceRule.
func ParseTriggerCondition(s string) (Condition, error) {
	fields := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return Condition{}, fmt.Errorf("alarms: malformed trigger condition segment %q", part)
		}
		fields[strings.ToLower(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
	}

	minutesStr, ok := fields["minutes"]
	if !ok {
		return Condition{}, fmt.Errorf("alarms: trigger condition missing minutes=N")
	}
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		return Condition{}, fmt.Errorf("alarms: invalid minutes value %q", minutesStr)
	}

	if name, ok := fields["category"]; ok && name != "" {
		return Condition{Scope: ScopeCategory, Name: name, Minutes: minutes}, nil
	}
	if name, ok := fields["project"]; ok && name != "" {
		return Condition{Scope: ScopeProject, Name: name, Minutes: minutes}, nil
	}
	return Condition{}, fmt.Errorf("alarms: trigger condition needs category=NAME or project=NAME")
}
