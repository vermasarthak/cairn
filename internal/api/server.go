package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/vermasarthak/cairn/internal/observability"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

type Server struct {
	Policies     *policy.PostgresStore
	Reservations *reservation.PostgresStore
	TokenTenants map[string]string // API key -> tenant ID; supplied only via process configuration.
	Now          func() time.Time
	Ready        func(context.Context) error
	Metrics      *observability.Metrics
	Logger       *slog.Logger
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", s.ready)
	if s.Metrics != nil {
		mux.Handle("GET /metrics", s.Metrics.Handler())
	}
	return mux
}

func (s Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.Ready != nil && s.Ready(r.Context()) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s Server) reserve(w http.ResponseWriter, r *http.Request) {
	tenant, ok := s.tenantFor(r.Header.Get("X-Cairn-API-Key"))
	if !ok {
		s.record("cairn_http_requests_unauthorized_total", http.StatusUnauthorized, "")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var request reservationRequest
	if err := decoder.Decode(&request); err != nil {
		s.record("cairn_http_requests_bad_request_total", http.StatusBadRequest, "")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	if request.PolicyID == "" || request.Subject.ID == "" || request.Subject.TimeZone == "" || request.Action.Key == "" {
		s.record("cairn_http_requests_bad_request_total", http.StatusBadRequest, tenant)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy_id, subject id, time zone, and action key are required"})
		return
	}
	configured, err := s.Policies.Latest(r.Context(), tenant, request.PolicyID)
	if err != nil {
		s.record("cairn_http_requests_policy_not_found_total", http.StatusNotFound, tenant)
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
		s.record("cairn_reservations_denied_total", http.StatusOK, tenant)
		writeJSON(w, http.StatusOK, map[string]any{"allowed": false, "reason": decision.Reason})
		return
	}
	result, err := s.Reservations.ReserveAndEnqueue(r.Context(), decision, subject, action)
	if err != nil {
		s.record("cairn_http_requests_error_total", http.StatusInternalServerError, tenant)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not reserve action"})
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
		s.record("cairn_reservations_created_total", status, tenant)
	} else {
		s.record("cairn_reservations_duplicate_total", status, tenant)
	}
	writeJSON(w, status, map[string]any{"allowed": true, "created": result.Created, "job_id": result.JobID, "local_day": decision.LocalDay, "policy_version": decision.PolicyVersion})
}

func (s Server) record(metric string, status int, tenant string) {
	if s.Metrics != nil {
		s.Metrics.Inc(metric)
	}
	if s.Logger != nil {
		s.Logger.Info("reservation request", "metric", metric, "status", status, "tenant", tenant)
	}
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
