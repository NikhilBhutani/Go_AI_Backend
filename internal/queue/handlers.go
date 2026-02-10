package queue

import (
	"github.com/hibiken/asynq"
)

type HandlersRegistry struct {
	mux *asynq.ServeMux
}

func NewHandlersRegistry() *HandlersRegistry {
	return &HandlersRegistry{
		mux: asynq.NewServeMux(),
	}
}

func (r *HandlersRegistry) Register(taskType string, handler asynq.Handler) {
	r.mux.Handle(taskType, handler)
}

func (r *HandlersRegistry) Mux() *asynq.ServeMux {
	return r.mux
}
