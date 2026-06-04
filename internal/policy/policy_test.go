package policy

import (
	"testing"
	"time"
)

func TestEvaluateUsesSubjectsLocalQuietHours(t *testing.T) {
	now := time.Date(2026, 9, 19, 17, 45, 0, 0, time.UTC) // 23:15 in Kolkata
	decision := Evaluate(Policy{ID: "daily", Version: 1, Active: true, QuietStartHour: 22, QuietEndHour: 9, MaxPerLocalDay: 1}, Subject{ID: "s1", TenantID: "t1", TimeZone: "Asia/Kolkata", Consented: true}, Action{Key: "check-in"}, nil, now)
	if decision.Allowed || decision.Reason != QuietHours {
		t.Fatalf("got %#v, want quiet-hours denial", decision)
	}
}

func TestEvaluateCapsPerLocalDay(t *testing.T) {
	now := time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	subject := Subject{ID: "s1", TenantID: "t1", TimeZone: "America/Los_Angeles", Consented: true}
	decision := Evaluate(Policy{ID: "daily", Version: 2, Active: true, QuietStartHour: 22, QuietEndHour: 9, MaxPerLocalDay: 1}, subject, Action{Key: "check-in"}, []Delivery{{SubjectID: "s1", PolicyID: "daily", ActionKey: "check-in", DeliveredAt: now.Add(-2 * time.Hour)}}, now)
	if decision.Allowed || decision.Reason != DailyCapReached {
		t.Fatalf("got %#v, want daily cap denial", decision)
	}
}

func TestEvaluateRejectsMissingConsent(t *testing.T) {
	decision := Evaluate(Policy{ID: "daily", Version: 1, Active: true, QuietStartHour: 22, QuietEndHour: 9, MaxPerLocalDay: 1}, Subject{ID: "s1", TenantID: "t1", TimeZone: "UTC"}, Action{Key: "check-in"}, nil, time.Now())
	if decision.Reason != MissingConsent {
		t.Fatalf("got %#v, want missing consent", decision)
	}
}
