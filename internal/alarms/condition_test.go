package alarms

import "testing"

func TestParseTriggerCondition_Category(t *testing.T) {
	c, err := ParseTriggerCondition("category=Gaming;minutes=60")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Scope != ScopeCategory || c.Name != "Gaming" || c.Minutes != 60 {
		t.Fatalf("got %+v", c)
	}
}

func TestParseTriggerCondition_Project(t *testing.T) {
	c, err := ParseTriggerCondition("project=Serenity;minutes=120")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Scope != ScopeProject || c.Name != "Serenity" || c.Minutes != 120 {
		t.Fatalf("got %+v", c)
	}
}

func TestParseTriggerCondition_OrderIndependent(t *testing.T) {
	c, err := ParseTriggerCondition("minutes=45;category=Music")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Scope != ScopeCategory || c.Name != "Music" || c.Minutes != 45 {
		t.Fatalf("got %+v", c)
	}
}

func TestParseTriggerCondition_MissingMinutes(t *testing.T) {
	if _, err := ParseTriggerCondition("category=Gaming"); err == nil {
		t.Fatal("expected an error when minutes is missing")
	}
}

func TestParseTriggerCondition_ZeroMinutesInvalid(t *testing.T) {
	if _, err := ParseTriggerCondition("category=Gaming;minutes=0"); err == nil {
		t.Fatal("expected an error for minutes=0 (a threshold of zero is meaningless)")
	}
}

func TestParseTriggerCondition_MissingScope(t *testing.T) {
	if _, err := ParseTriggerCondition("minutes=30"); err == nil {
		t.Fatal("expected an error when neither category nor project is present")
	}
}

func TestParseTriggerCondition_NonNumericMinutes(t *testing.T) {
	if _, err := ParseTriggerCondition("category=Gaming;minutes=abc"); err == nil {
		t.Fatal("expected an error for a non-numeric minutes value")
	}
}
