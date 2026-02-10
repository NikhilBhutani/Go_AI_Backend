package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/nikhilbhutani/backendwithai/internal/api/handlers"
	"github.com/nikhilbhutani/backendwithai/internal/api/middleware"
	"github.com/nikhilbhutani/backendwithai/internal/audit"
	"github.com/nikhilbhutani/backendwithai/internal/auth"
	"github.com/nikhilbhutani/backendwithai/internal/config"
	"github.com/nikhilbhutani/backendwithai/internal/document"
	"github.com/nikhilbhutani/backendwithai/internal/embedding"
	"github.com/nikhilbhutani/backendwithai/internal/finetune"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/prompt"
	"github.com/nikhilbhutani/backendwithai/internal/queue"
	"github.com/nikhilbhutani/backendwithai/internal/rag"
	"github.com/nikhilbhutani/backendwithai/internal/storage"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
	"github.com/nikhilbhutani/backendwithai/internal/webhook"
)

type Router struct {
	mux    *chi.Mux
	db     *pgxpool.Pool
	redis  *redis.Client
	cfg    *config.Config
	ts     *tenant.Service
	jwt    *auth.JWTMiddleware
	apikey *auth.APIKeyMiddleware
	rbac   *auth.RBAC
	llmGW  llm.Gateway
}

func NewRouter(db *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *Router {
	ts := tenant.NewService(db)
	return &Router{
		mux:    chi.NewRouter(),
		db:     db,
		redis:  rdb,
		cfg:    cfg,
		ts:     ts,
		jwt:    auth.NewJWTMiddleware(cfg.Auth.JWTSecret, ts),
		apikey: auth.NewAPIKeyMiddleware(db, cfg.Auth.APIKeyHeader, ts),
		rbac:   auth.NewRBAC(db),
		llmGW:  llm.NewGateway(cfg.LLM),
	}
}

func (rt *Router) Setup() http.Handler {
	r := rt.mux

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS([]string{"*"}))

	rl := middleware.NewRateLimiter(100, 200)
	r.Use(rl.Limit)

	// Health endpoints (no auth)
	health := handlers.NewHealthHandler(rt.db, rt.redis)
	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)

	// Initialize services
	store := storage.NewSupabaseStorage(rt.cfg.Storage.SupabaseURL, rt.cfg.Storage.SupabaseKey)
	docSvc := document.NewService(rt.db, store, rt.cfg.Storage.Bucket)
	promptSvc := prompt.NewService(rt.db)
	auditSvc := audit.NewService(rt.db)
	queueClient := queue.NewClient(rt.cfg.Redis)
	dispatcher := webhook.NewDispatcher(rt.db)
	webhookSvc := webhook.NewService(rt.db, dispatcher)

	vs := vectorstore.NewPgVectorStore(rt.db)
	embedSvc := embedding.NewService(rt.llmGW, "")
	ragPipeline := rag.NewPipeline(vs, embedSvc, rt.llmGW)

	finetuneRegistry := finetune.NewRegistry(rt.db)
	finetuneSvc := finetune.NewService(rt.db, store, rt.cfg.Storage.Bucket, finetuneRegistry, queueClient)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth: try API key first, then JWT
		r.Use(rt.apikey.Authenticate)
		r.Use(rt.jwt.Authenticate)

		// LLM routes
		llmH := handlers.NewLLMHandler(rt.llmGW)
		r.Route("/llm", func(r chi.Router) {
			r.Post("/chat", llmH.Chat)
			r.Post("/chat/stream", llmH.ChatStream)
			r.Post("/embed", llmH.Embed)
			r.Get("/models", llmH.Models)
		})

		// Document routes
		docH := handlers.NewDocumentHandler(docSvc)
		r.Route("/documents", func(r chi.Router) {
			r.Post("/", docH.Upload)
			r.Get("/", docH.List)
			r.Get("/{id}", docH.Get)
			r.Delete("/{id}", docH.Delete)
			r.Get("/{id}/status", docH.Status)
		})

		// RAG routes
		ragH := handlers.NewRAGHandler(ragPipeline)
		r.Route("/rag", func(r chi.Router) {
			r.Post("/query", ragH.Query)
			r.Post("/search", ragH.Search)
		})

		// Prompt routes
		promptH := handlers.NewPromptHandler(promptSvc)
		r.Route("/prompts", func(r chi.Router) {
			r.Post("/", promptH.Create)
			r.Get("/", promptH.List)
			r.Get("/{id}", promptH.Get)
			r.Put("/{id}", promptH.CreateVersion)
			r.Post("/{id}/render", promptH.RenderPrompt)
		})

		// Finetune routes
		finetuneH := handlers.NewFinetuneHandler(finetuneSvc)
		r.Route("/finetune", func(r chi.Router) {
			r.Post("/datasets", finetuneH.UploadDataset)
			r.Get("/datasets", finetuneH.ListDatasets)
			r.Post("/jobs", finetuneH.StartJob)
			r.Get("/jobs", finetuneH.ListJobs)
			r.Get("/jobs/{id}", finetuneH.GetJob)
			r.Get("/models", finetuneH.ListModels)
		})

		// Webhook routes
		webhookH := handlers.NewWebhookHandler(webhookSvc)
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/", webhookH.Create)
			r.Get("/", webhookH.List)
			r.Delete("/{id}", webhookH.Delete)
		})

		// Admin routes
		adminH := handlers.NewAdminHandler(auditSvc)
		r.Route("/admin", func(r chi.Router) {
			r.Get("/usage", adminH.Usage)
			r.Get("/audit", adminH.AuditLogs)
		})

		// Agent routes
		agentH := handlers.NewAgentHandler(rt.llmGW)
		r.Route("/agents", func(r chi.Router) {
			r.Post("/run", agentH.Run)
			r.Post("/chain", agentH.Chain)
		})

		// Guardrails routes
		guardrailH := handlers.NewGuardrailHandler(rt.llmGW)
		r.Route("/guardrails", func(r chi.Router) {
			r.Post("/check", guardrailH.Check)
			r.Post("/classify", guardrailH.Classify)
		})

		// Eval routes
		evalH := handlers.NewEvalHandler(rt.llmGW)
		r.Route("/eval", func(r chi.Router) {
			r.Post("/suite", evalH.RunSuite)
			r.Post("/judge", evalH.Judge)
			r.Post("/compare", evalH.Compare)
		})

		// Reasoning routes
		reasoningH := handlers.NewReasoningHandler(rt.llmGW)
		r.Route("/reasoning", func(r chi.Router) {
			r.Post("/cot", reasoningH.ChainOfThought)
			r.Post("/tot", reasoningH.TreeOfThought)
			r.Post("/self-consistency", reasoningH.SelfConsistency)
			r.Post("/reflect", reasoningH.Reflect)
		})

		// Multimodal routes
		multimodalH := handlers.NewMultimodalHandler(rt.llmGW, rt.cfg.LLM.OpenAIKey)
		r.Route("/multimodal", func(r chi.Router) {
			r.Post("/vision", multimodalH.Analyze)
			r.Post("/image/generate", multimodalH.GenerateImage)
			r.Post("/tts", multimodalH.Speak)
		})
	})

	return r
}
