package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: rdb}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}

	if h.db != nil {
		if err := h.db.Ping(r.Context()); err != nil {
			checks["database"] = "unhealthy: " + err.Error()
		} else {
			checks["database"] = "ok"
		}
	}

	if h.redis != nil {
		if err := h.redis.Ping(r.Context()).Err(); err != nil {
			checks["redis"] = "unhealthy: " + err.Error()
		} else {
			checks["redis"] = "ok"
		}
	}

	status := http.StatusOK
	for _, v := range checks {
		if v != "ok" {
			status = http.StatusServiceUnavailable
			break
		}
	}

	writeJSON(w, status, map[string]interface{}{"status": statusStr(status), "checks": checks})
}

func statusStr(code int) string {
	if code == http.StatusOK {
		return "ok"
	}
	return "unhealthy"
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
