package policy

import (
	"fmt"
	"time"
)

type Subject struct {
	ID        string
	TenantID  string
	TimeZone  string
	Consented bool
}

type Action struct {
	Key string
}

type Policy struct {
	ID             string
	Version        int
	Active         bool
	QuietStartHour int
	QuietEndHour   int
	MaxPerLocalDay int
}

type Delivery struct {
	SubjectID   string
	PolicyID    string
	ActionKey   string
	DeliveredAt time.Time
}

type Reason string

const (
	Allowed         Reason = "allowed"
	PolicyInactive  Reason = "policy_inactive"
	MissingConsent  Reason = "missing_consent"
	QuietHours      Reason = "quiet_hours"
	DailyCapReached Reason = "daily_cap_reached"
	InvalidTimeZone Reason = "invalid_time_zone"
	InvalidPolicy   Reason = "invalid_policy"
)

type Decision struct {
	Allowed       bool
	Reason        Reason
	PolicyID      string
	PolicyVersion int
	EvaluatedAt   time.Time
	LocalDay      string
}

func Evaluate(p Policy, subject Subject, action Action, deliveries []Delivery, now time.Time) Decision {
	decision := Decision{PolicyID: p.ID, PolicyVersion: p.Version, EvaluatedAt: now.UTC()}
	if err := p.validate(); err != nil || subject.ID == "" || subject.TenantID == "" || action.Key == "" {
		decision.Reason = InvalidPolicy
		return decision
	}
	location, err := time.LoadLocation(subject.TimeZone)
	if err != nil {
		decision.Reason = InvalidTimeZone
		return decision
	}
	localNow := now.In(location)
	decision.LocalDay = localNow.Format("2006-01-02")
	if !p.Active {
		decision.Reason = PolicyInactive
		return decision
	}
	if !subject.Consented {
		decision.Reason = MissingConsent
		return decision
	}
	if p.inQuietHours(localNow.Hour()) {
		decision.Reason = QuietHours
		return decision
	}
	count := 0
	for _, delivery := range deliveries {
		if delivery.SubjectID != subject.ID || delivery.PolicyID != p.ID || delivery.ActionKey != action.Key {
			continue
		}
		if delivery.DeliveredAt.In(location).Format("2006-01-02") == decision.LocalDay {
			count++
		}
	}
	if count >= p.MaxPerLocalDay {
		decision.Reason = DailyCapReached
		return decision
	}
	decision.Allowed = true
	decision.Reason = Allowed
	return decision
}

func (p Policy) validate() error {
	if p.ID == "" || p.Version < 1 || p.MaxPerLocalDay < 1 {
		return fmt.Errorf("policy id, positive version, and positive daily cap are required")
	}
	if p.QuietStartHour < 0 || p.QuietStartHour > 23 || p.QuietEndHour < 0 || p.QuietEndHour > 23 || p.QuietStartHour == p.QuietEndHour {
		return fmt.Errorf("quiet hours must be distinct hours from 0 through 23")
	}
	return nil
}

func (p Policy) inQuietHours(hour int) bool {
	if p.QuietStartHour < p.QuietEndHour {
		return hour >= p.QuietStartHour && hour < p.QuietEndHour
	}
	return hour >= p.QuietStartHour || hour < p.QuietEndHour
}
