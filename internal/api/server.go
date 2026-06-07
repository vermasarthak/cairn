package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

type Server struct {
	Policies     *policy.PostgresStore
	Reservations *reservation.PostgresStore
	TokenTenants map[string]string // API key -> tenant ID; supplied only via process configuration.
	Now          func() time.Time
}

type reservationRequest struct {
	PolicyID string `json:"policy_id"`
	Subject  struct {
		ID        string `json:"id"`
		TimeZone  string `json:"time_zone"`
		Consented bool   `json:"consented"`
	} `json:"subject"`
	Action struct {
		Key string `json:"key"`
	} `json:"action"`
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/reservations", s.reserve)
	return mux
}

func (s Server) reserve(w http.ResponseWriter, r *http.Request) {
	tenant, ok := s.tenantFor(r.Header.Get("X-Cairn-API-Key"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var request reservationRequest
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	if request.PolicyID == "" || request.Subject.ID == "" || request.Subject.TimeZone == "" || request.Action.Key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy_id, subject id, time zone, and action key are required"})
		return
	}
	configured, err := s.Policies.Latest(r.Context(), tenant, request.PolicyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
		return
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	subject := policy.Subject{ID: request.Subject.ID, TenantID: tenant, TimeZone: request.Subject.TimeZone, Consented: request.Subject.Consented}
	action := policy.Action{Key: request.Action.Key}
	decision := policy.Evaluate(configured, subject, action, nil, now)
	if !decision.Allowed {
		writeJSON(w, http.StatusOK, map[string]any{"allowed": false, "reason": decision.Reason})
		return
	}
	result, err := s.Reservations.ReserveAndEnqueue(r.Context(), decision, subject, action)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not reserve action"})
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"allowed": true, "created": result.Created, "job_id": result.JobID, "local_day": decision.LocalDay, "policy_version": decision.PolicyVersion})
}

func (s Server) tenantFor(given string) (string, bool) {
	if given == "" {
		return "", false
	}
	for token, tenant := range s.TokenTenants {
		if subtle.ConstantTimeCompare([]byte(given), []byte(token)) == 1 {
			return tenant, true
		}
	}
	return "", false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
