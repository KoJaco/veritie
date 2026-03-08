package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	gstt "cloud.google.com/go/speech/apiv1/speechpb"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"schma.ai/internal/app/connection"
	"schma.ai/internal/app/llmprovider"
	"schma.ai/internal/app/llmworker"
	"schma.ai/internal/app/pipeline"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/domain/usage"
	db "schma.ai/internal/infra/db/generated"
	"schma.ai/internal/infra/db/repo"
	"schma.ai/internal/infra/fastparser"
	infra_redaction "schma.ai/internal/infra/redaction"
	"schma.ai/internal/infra/sttfactory"
	"schma.ai/internal/infra/sttgoogle"
	infra_normalizer "schma.ai/internal/infra/sttnormalizer"
	"schma.ai/internal/infra/sttrouter"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/transport/ws"

	auth_app "schma.ai/internal/app/auth"
	"schma.ai/internal/app/batch"
	auth_infra "schma.ai/internal/infra/auth"
	"schma.ai/internal/infra/session"
	http_transport "schma.ai/internal/transport/http"
	http_middleware "schma.ai/internal/transport/http/middleware"

	sttdeepgram "schma.ai/internal/infra/sttdeepgram"
)

// TODO: move these helpers elsewhere
// for fly deployment, keep sensitive info out of the image
func initServiceAccount() {
	// 1. check JSON content is provided as a secret.
	if jsonStr := os.Getenv("GOOGLE_STT_SERVICE_ACCOUNT_JSON"); jsonStr != "" {
		// 2. define a file path to write credentials
		filePath := "/tmp/service_account_secret.json"
		// 3. write the secret's content to the file
		if err := os.WriteFile(filePath, []byte(jsonStr), 0644); err != nil {
			logger.Errorf("❌ [STARTUP] Failed to write service account file: %v", err)
			os.Exit(1)
		}

		// 4. Update the environment variable to point to the newly written file.
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filePath)
		logger.Infof("✅ [STARTUP] Service account credentials written to %s", filePath)

	} else {
		logger.Infof("ℹ️ [STARTUP] Development mode: GOOGLE_STT_SERVICE_ACCOUNT_JSON not set")
	}
}

func ModelDir() string {
	if d := os.Getenv("MODEL_DIR"); d != "" {
		return d
	}
	return "/data/models" // default for Fly volume
}

func RuntimeDir() string {
	if d := os.Getenv("ONNX_RUNTIME_PATH"); d != "" {
		return d
	}
	return "/usr/lib" // default for Fly volume
}

func GoogleCredsDir() string {
	if d := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); d != "" {
		return d
	}
	return "/etc/creds" // default for Fly volume
}

// getenvDefault returns env[key] or def if unset.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIsProd() bool {
	v := strings.ToLower(os.Getenv("GO_ENV")) 
	return v == "prod" || v == "production"
}

// Comma-separated allowlist with optional wildcards, e.g.:
// ALLOWED_ORIGINS="https://schma.ai,https://admin.schma.ai,https://*.vercel.app"
func parseAllowedOrigins() []string {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		// sane defaults for prod+previews; override via env
		raw = "https://schma.ai,https://www.schma.ai,https://admin.schma.ai,http://localhost:*"
		// raw = "https://schma.ai,https://admin.schma.ai,https://*.vercel.app,http://localhost:*"
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		// strip any trailing "/*" someone might add out of habit
		s = strings.TrimSuffix(s, "/*")
		s = strings.TrimSuffix(s, "/")
		if s != "" { out = append(out, s) }
	}
	return out
}

// match origin "scheme://host[:port]" against patterns (supports "*.domain.tld")
func originAllowed(origin string, patterns []string, dev bool) bool {
	if origin == "" { return false }
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" { return false }

	host := u.Hostname()
	scheme := u.Scheme

	// dev shortcuts
	if !dev {
		// prod must match patterns
	} else {
		// allow popular local dev origins
		if (scheme == "http" || scheme == "https") && (host == "localhost" || host == "127.0.0.1") {
			return true
		}
	}

	for _, pat := range patterns {
		pu, err := url.Parse(pat)
		if err != nil || pu.Scheme == "" { continue }
		if pu.Scheme != scheme { continue }

		ppHost := pu.Hostname()
		if strings.HasPrefix(ppHost, "*.") {
			// wildcard subdomain match
			suffix := strings.TrimPrefix(ppHost, "*.")
			if strings.HasSuffix(host, "."+suffix) || host == suffix {
				return true
			}
		} else if host == ppHost {
			return true
		}
	}
	return false
}


// --- FastText discovery helpers ---
func fastTextModelPath(base string) string {
    if p := os.Getenv("FASTTEXT_PATH"); p != "" { // explicit override
        if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() { return p }
        logger.Warnf("⚠️  [FASTTEXT] FASTTEXT_PATH is set but not a file: %s", p)
        return ""
    }
    // probe a few common filenames
    candidates := []string{
        filepath.Join(base, "fasttext", "cc.en.300.100k.vec"),   // quantized (ideal)
        filepath.Join(base, "fasttext", "cc.en.300.bin"),   // ~4–7 GB
        filepath.Join(base, "fasttext", "wiki-news-300d-1M.bin"),
        filepath.Join(base, "fasttext", "wiki-news-300d-1M.vec"),
    }
    for _, c := range candidates {
        if st, err := os.Stat(c); err == nil && st.Mode().IsRegular() { return c }
    }
    return ""
}


func main() {

	// MACHINE SPIN UP STAGE

	// 1. Load .env variables (dev conv only)
	_ = godotenv.Load(".env")

	// 1.1 Setup logging
	logger.SetLevelFromEnv(os.Getenv("LOG_LEVEL"))
	logger.InitDebugServices()

	// 2. Build core adapters

	// 2.1 connect to DB with connection pooling
	pool, err := pgxpool.New(context.Background(), os.Getenv("SUPABASE_DATABASE_URL"))

	if err != nil {
		logger.Errorf("❌ [STARTUP] Failed to create database pool: %v", err)
		os.Exit(1)
	}

	defer pool.Close()

	// ping db to test connection
	var version string
	if err := pool.QueryRow(context.Background(), "SELECT version()").Scan(&version); err != nil {
		logger.Errorf("❌ [STARTUP] Database query failed: %v", err)
		os.Exit(1)
	}

	logger.Infof("✅ [STARTUP] Connected to database: %s", version)

	// TODO: implement fallback based on concurrent requests

	// 2.2 Setup Google STT (still using as fallback)
	goEnv := os.Getenv("GO_ENV")
	if goEnv == "production" || goEnv == "staging" {
		initServiceAccount()
	} else {
		logger.Infof("🚀 [STARTUP] STT development mode: assuming local service account file will be set")
	}

	googleSttCfg := sttgoogle.Config{
		Encoding:        gstt.RecognitionConfig_WEBM_OPUS,
		SampleRateHertz: 16000,
		LanguageCode:    "en-US",
		Punctuate:       true,
	}

	// 2.3 Fast-parser (ONNX + fastText)
	// modelDir := filepath.Join(ModelDir(), "bge", "model.int8.onnx")
	modelDir := ModelDir()
	runtimeDir := RuntimeDir()
	fastTextDir := ModelDir()
	synOn := os.Getenv("SCHMA_NEIGHBOURS") == "0"

	logger.Infof("🚀 [STARTUP] all paths loaded: modelDir=%s, runtimeDir=%s, fastTextDir=%s, synOn=%t", modelDir, runtimeDir, fastTextDir, synOn)

	fpAdapter := fastparser.NewAdapter(modelDir, runtimeDir, fastTextDir, synOn)
	logger.Infof("🚀 [STARTUP] FastParser adapter created, warming up...")
	fpAdapter.Warmup() // eager load ONNX + (optional) fasttext

	if synOn {
		logger.Infof("🚀 [STARTUP] Loading FastText model at startup and testing synonyms")
		fpAdapter.NeighbourSynonyms("intellect", 5)
	}

	logger.Infof("🚀 [STARTUP] FastParser adapter warmed up, and ONNX should be set")

	// 2.4 LLM provider (v1 default; v2 when built with -tags=genai2)
	baseLLM, err := llmprovider.ProvideLLM(os.Getenv("GEMINI_API_KEY"), "gemini-2.0-flash")
	if err != nil {
		logger.Errorf("❌ [STARTUP] Failed to init LLM provider: %v", err)
		os.Exit(1)
	}

	logger.Infof("🚀 [STARTUP] LLM provider initialized")

	// Wrap LLM with worker pool adapter for isolated, retryable processing
	numWorkers := 4 // Default to 4 workers, can be made configurable
	llm := llmworker.NewWorkerPoolAdapter(baseLLM, numWorkers)
	
	// Start the worker pool
	if err := llm.Start(context.Background()); err != nil {
		logger.Errorf("❌ [STARTUP] Failed to start LLM worker pool: %v", err)
		os.Exit(1)
	}

	logger.Infof("🧠 [STARTUP] Gemini client initialized with worker pool (%d workers)", numWorkers)


	// 2.5 Initialize normalizer
	// TODO: extrapolate out UDS paths. Should have all our paths (runtime, model, uds, etc) all init via env vars with an init function helper
	normCli := infra_normalizer.New(infra_normalizer.Config{UDSPath: "/tmp/norm.sock"})
	norm := infra_normalizer.NewBatcher(infra_normalizer.BatcherConfig{
		Client: normCli,
		MaxBatch: 16, // default 16
		Window: 3 * time.Millisecond, // default 3ms
		})	
		
	texts := []string{
		"see you on eleven slash zero three slash twenty twenty four at half past four",
		"his mobile is oh four one two three four five six seven eight",
	}
	
	// Test init
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	outs, err := norm.NormalizeBatch(ctx, texts)
	if err != nil {
		logger.Errorf("❌ [STARTUP] Failed to normalize chunk: %v", err)
	} else {
		logger.Infof("✅ [STARTUP] Normalizer initialized: %s", outs)
	}
	cancel()

	// 2.4 Initialize redaction service

	// Create the infra redactor that implements the domain interface
	phiRedactor, err := infra_redaction.NewPHIRedactor(
		filepath.Join(ModelDir(), "phi_roberta_onnx_int8", "model_quantized.onnx"),
		filepath.Join(ModelDir(), "phi_roberta_onnx_int8", "config.json"),
		"/tmp/tok.sock",
	)

	if err != nil {
		logger.Errorf("❌ [STARTUP] Failed to initialize PHI redactor: %v", err)
		os.Exit(1)
	}

	logger.Infof("🔒 [STARTUP] PHI redactor initialized")

	// 2.5 Initialize auth components
	authRepo := repo.NewAuthRepo(pool)
	cache, err := auth_infra.NewAppSettingsCache(1000) // 1000 entries
	if err != nil {
		logger.Errorf("❌ [STARTUP] Failed to create auth cache: %v", err)
		os.Exit(1)
	}
	

	// 2.5 Setup Rate Limiting
	rateLimit, err := strconv.ParseInt(os.Getenv("RATE_LIMIT"), 10, 64)

	if err != nil {
		logger.Warnf("⚠️ [STARTUP] Failed to parse RATE_LIMIT, using default of 20 requests per minute per app: %v", err)
		rateLimit = 20
	}

	rateLimiter := auth_infra.NewRateLimiter(rateLimit)

	// 2.5 Initialize auth service
	authService := auth_app.NewService(authRepo, cache, rateLimiter)

	// 2.6 Initialize session manager (1 hour TTL)
	sessionManager := auth_infra.NewMemorySessionManager(1 * time.Hour)

	// 2.7 Initialize database session manager
	dbSessionManager := session.New(db.New(pool))

	// 2.8 Initialize connection service
	connectionRepo := repo.NewConnectionRepo(pool)
	connectionService := connection.NewService(connectionRepo, 200, 30*time.Second) // 1000 max connections, 30s cleanup
	
	// Start connection service
	if err := connectionService.Start(context.Background()); err != nil {
		logger.Errorf("❌ [STARTUP] Failed to start connection service: %v", err)
		os.Exit(1)
	}
	logger.Infof("✅ [STARTUP] Connection service started successfully")

	// Graceful shutdown for LLM worker pool
	defer llm.Stop()
	
	// Graceful shutdown for connection service
	defer func() {
		if err := connectionService.Stop(context.Background()); err != nil {
			logger.Errorf("❌ [SHUTDOWN] Failed to stop connection service: %v", err)
		}
	}()

	// 3. Bundle deps and transport
	pricing := usage.DefaultPricing

	sttProvider := os.Getenv("SCHMA_STT_PROVIDER")

	if sttProvider == "deepgram" {
		pricing.CostAudioPerMin = 0.0077

	} else {
		pricing.CostAudioPerMin = 0.016
	}

	logger.Infof("💰 [STARTUP] Pricing configuration: %+v", pricing)
	// 4.0 Init repos for db trx

	// 4.1 Initialize usage repositories
	usageMeterRepo := repo.NewMeterRepo(pool)
	usageEventRepo := repo.NewUsageEventRepo(pool)
	draftAggRepo := repo.NewDraftAggRepo(pool)

	// 4.2 Initialize session data repositories
	functionCallsRepo := repo.NewFunctionCallsRepo(pool)
	functionSchemasRepo := repo.NewFunctionSchemasRepo(pool)
	transcriptsRepo := repo.NewTranscriptsRepo(pool)
	structuredOutputsRepo := repo.NewStructuredOutputsRepo(pool)
	structuredOutputSchemasRepo := repo.NewStructuredOutputSchemasRepo(pool)

	// 4.3 Initialize and startup batch components
	batchRepo := repo.NewBatchRepo(pool)
	// Use Deepgram BatchClient (pre-recorded) for batch processing
	deepgramBatch := sttdeepgram.NewBatchClient(os.Getenv("DEEPGRAM_API_KEY"), "nova-3", speech.DiarizationConfig{EnableSpeakerDiarization: false})
	batchProcessor := batch.NewBatchProcessor(batchRepo, deepgramBatch, llm, transcriptsRepo, functionCallsRepo, structuredOutputsRepo, functionSchemasRepo, structuredOutputSchemasRepo, usageMeterRepo, usageEventRepo, draftAggRepo, dbSessionManager, norm, phiRedactor, pricing)
	batchQueueManager := batch.NewQueueManager(batchRepo, batchProcessor, 2)

	// Start batch queue manager
	go func() {
		if err := batchQueueManager.Start(context.Background()); err != nil {
			logger.Errorf("❌ [BATCH] Failed to start batch queue manager: %v", err)
		}
	}()

	// Graceful shutdown for batch manager
	defer batchQueueManager.Stop()

	// TODO: rethink stt stuff before this, need to be selectable by the client with a config var
	// 4.4 Create STT factory for per-session provider selection
	sttFactory := sttfactory.NewFactory(googleSttCfg, os.Getenv("DEEPGRAM_API_KEY"))

	// 5. Create pipeline dependencies
	deps := pipeline.Deps{
		// Speech-to-text
		STT: sttrouter.New(ctx, googleSttCfg, speech.DiarizationConfig{
			EnableSpeakerDiarization: false,
		}), // Fallback/default STT client
		STTFactory: sttFactory, // Factory for per-session STT clients
		// Fast Inference 'draft' function calls
		FP: fpAdapter,
		// Main Nuanced NLU System
		LLM: llm,
		// Text Normalizer
		Normalizer: norm,
		// Redaction service - using infra PHI redactor
		RedactionService: phiRedactor,
		// Usage Repos per session
		UsageMeterRepo: usageMeterRepo,
		UsageEventRepo: usageEventRepo,
		DraftAggRepo:   draftAggRepo,
		// Explicit session data (very valuable for rebuilding sessions retroactively and for further model training)
		FunctionCallsRepo:           functionCallsRepo,
		FunctionSchemasRepo:         functionSchemasRepo,
		StructuredOutputsRepo:       structuredOutputsRepo,
		StructuredOutputSchemasRepo: structuredOutputSchemasRepo,
		TranscriptsRepo:             transcriptsRepo,
	}
	
	logger.Infof("🔒 [STARTUP] Pipeline deps created with PHI redactor: %+v", deps.RedactionService)

	// 6. Handlers for HTTP and WS

	// build upgrader
	allowed := parseAllowedOrigins()
	dev := !envIsProd()
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		EnableCompression: false, // disable while debugging
		Subprotocols: []string{"schma.ws.v1"},
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return originAllowed(origin, allowed, dev)
		},
	}

	// 6.1 oinit WS handler and Apply middleware
	// authenticatedWS := http_middleware.SessionAuthMiddleware(sessionManager)(wsHandler)

	// 6.2 Create batch handler and apply http middleware
	batchHandler := http_transport.NewBatchHandler(batchRepo, batchQueueManager, dbSessionManager, functionSchemasRepo, structuredOutputSchemasRepo)
	authenticatedBatchUpload := http_middleware.KeyAuthMiddleware(authService)(http.HandlerFunc(batchHandler.HandleUpload))
	authenticatedBatchList := http_middleware.KeyAuthMiddleware(authService)(http.HandlerFunc(batchHandler.HandleListJobs))

	// 6.3 Create auth handler for session creation and apply middleware
	authHandler := http_transport.NewAuthHandler(sessionManager)
	logger.Infof("🔒 [STARTUP] Auth handler initialized")
	authenticatedAuth := http_middleware.KeyAuthMiddleware(authService)(http.HandlerFunc(authHandler.HandleAuth))
	logger.Infof("🔒 [STARTUP] Authenticated auth handler initialized")

	// 6.4 Create health checker
	healthChecker := http_transport.NewHealthChecker(pool, deps.STT)


	// Wait loop before serving traffic (if we want to enforce text normalization)
	// Todo: make this configurable
	deadline := time.Now().Add(10 * time.Second)
	for {
		if norm.Healthy(context.Background()) {break}
		if time.Now().After(deadline) { log.Fatal("norm sidecar not ready")}
		time.Sleep(200 * time.Millisecond)
	}


	// 6.5 Serve HTTP
	mux := http.NewServeMux()
	// mux.Handle("/ws", authenticatedWS)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.NewHandler(deps, pricing, upgrader, db.New(pool), dbSessionManager, authService, connectionService).ServeHTTP(w, r)
	}) // Session auth, WS endpoint
	// mux.Handle("/api/auth", authenticatedAuth)  // old auth
	mux.Handle("/api/v1/tokens/ws", authenticatedAuth)                                    // API key auth → session
	mux.Handle("/api/v1/batch", authenticatedBatchUpload)                           // API key auth → batch upload
	mux.Handle("/api/v1/batch/status", http.HandlerFunc(batchHandler.HandleGetJob)) // API key auth → batch status
	mux.Handle("/api/v1/batch/jobs", authenticatedBatchList)                        // API key auth → batch list
	mux.HandleFunc("/healthz", healthChecker.Healthz)                               // health check (see fly.toml)
	mux.HandleFunc("/readyz", healthChecker.Readyz)                                 // readiness check (see fly.toml)

	// Start batch temp janitor (sweeps old /tmp batch dirs). Safe to run alongside HTTP server.
	startBatchTempJanitor(context.Background())

	// 7. Post HTTP init config, protect, and cleanup

	// 7.1 Wrap the entire mux with CORS middleware
	corsHandler := http_middleware.DevelopmentCORS(mux)

	// 7.2. HTTP server with graceful shutdown

	addr := getenvDefault("PORT", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           corsHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// 7.3 Start HTTP server and listen for shutdown signals
	go func() {
		logger.Infof("🚀 [STARTUP] Schma server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("❌ [STARTUP] Server failed to start: %v", err)
			os.Exit(1)
		}
	}()

	// 7.4 Shutdown server and cancel context -- graceful shutdown
	<-stop
	logger.Infof("🛑 [STARTUP] Shutting down server...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)

}

// startBatchTempJanitor launches a background janitor to remove old temp batch files/dirs.
// It sweeps both the upload dir (/tmp/schma-batch) and the artifacts dir (/tmp/schma-batch-results).
// TTL can be configured via BATCH_TEMP_TTL_MINUTES (default 120 minutes). Sweep interval via BATCH_JANITOR_INTERVAL_MINUTES (default 30).
func startBatchTempJanitor(ctx context.Context) {
	// Read TTL and interval from env with defaults
	ttlMin := int64(120)
	if v, err := strconv.ParseInt(os.Getenv("BATCH_TEMP_TTL_MINUTES"), 10, 64); err == nil && v > 0 {
		ttlMin = v
	}
	intervalMin := int64(30)
	if v, err := strconv.ParseInt(os.Getenv("BATCH_JANITOR_INTERVAL_MINUTES"), 10, 64); err == nil && v > 0 {
		intervalMin = v
	}

	ttl := time.Duration(ttlMin) * time.Minute
	interval := time.Duration(intervalMin) * time.Minute

	uploadRoot := filepath.Join(os.TempDir(), "schma-batch")
	resultsRoot := filepath.Join(os.TempDir(), "schma-batch-results")

	sweep := func(root string) {
		d, err := os.ReadDir(root)
		if err != nil {
			return
		}
		cutoff := time.Now().Add(-ttl)
		for _, e := range d {
			if !e.IsDir() { continue }
			p := filepath.Join(root, e.Name())
			fi, err := os.Stat(p)
			if err != nil { continue }
			// Remove if last mod before cutoff
			if fi.ModTime().Before(cutoff) {
				_ = os.RemoveAll(p)
			}
		}
	}

	go func() {
		// initial sweep
		sweep(uploadRoot)
		sweep(resultsRoot)
		// periodic sweeps
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sweep(uploadRoot)
				sweep(resultsRoot)
			}
		}
	}()
}
