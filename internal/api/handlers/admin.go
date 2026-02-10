package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/nikhilbhutani/backendwithai/internal/audit"
)

type AdminHandler struct {
	auditSvc *audit.Service
}

func NewAdminHandler(auditSvc *audit.Service) *AdminHandler {
	return &AdminHandler{auditSvc: auditSvc}
}

func (h *AdminHandler) Usage(w http.ResponseWriter, r *http.Request) {
	var startDate, endDate *time.Time

	if s := r.URL.Query().Get("start_date"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			startDate = &t
		}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			endDate = &t
		}
	}

	summary, err := h.auditSvc.GetUsageSummary(r.Context(), startDate, endDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"usage": summary})
}

func (h *AdminHandler) AuditLogs(w http.ResponseWriter, r *http.Request) {
	q := audit.AuditQuery{
		Action: r.URL.Query().Get("action"),
	}

	q.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	q.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if q.Limit <= 0 {
		q.Limit = 50
	}

	if s := r.URL.Query().Get("start_date"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			q.StartDate = &t
		}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			q.EndDate = &t
		}
	}

	logs, err := h.auditSvc.GetAuditLogs(r.Context(), q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"audit_logs": logs, "count": len(logs)})
}
