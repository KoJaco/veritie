package logger

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Entry struct {
	T time.Time
	Level Level
	Msg string
	KVs map[string]any
}

type Span struct {
	Name string
	KVs map[string]any
	buf []Entry
	mu sync.Mutex
	start time.Time
	maxBuf int
}

var current = LevelInfo

// Service-specific debug control
var debugServices = make(map[string]bool)
var debugServicesMu sync.RWMutex

// Tunables for span buffering and flush
var spanTail = 40
var spanMaxBuf = 200

// InitDebugServices initializes service-specific debug logging from environment
func InitDebugServices() {
	// Parse DEBUG_SERVICES environment variable
	// Format: "DEEPGRAM,WS,LLM" or "ALL" for all services
	debugEnv := os.Getenv("DEBUG_SERVICES")
	if debugEnv == "" {
		return
	}
	
	services := strings.Split(debugEnv, ",")
	log.Printf("INFO: Initializing debug services: %v", services)
	for _, service := range services {
		service = strings.TrimSpace(strings.ToUpper(service))
		if service == "ALL" {
			// Enable debug for all services
			debugServicesMu.Lock()
			debugServices["*"] = true
			debugServicesMu.Unlock()
			return
		}
		if service != "" {
			debugServicesMu.Lock()
			debugServices[service] = true
			debugServicesMu.Unlock()
		}
	}
}

// IsServiceDebugEnabled checks if debug logging is enabled for a specific service
func IsServiceDebugEnabled(service string) bool {
	debugServicesMu.RLock()
	defer debugServicesMu.RUnlock()
	
	// Check if all services are enabled
	if debugServices["*"] {
		return true
	}
	
	// Check if specific service is enabled
	return debugServices[strings.ToUpper(service)]
}

// ServiceDebugf logs debug messages only if the service is enabled for debug
func ServiceDebugf(service, format string, v ...interface{}) {
	if IsServiceDebugEnabled(service) {
		// Prepend service name to the format string
		serviceFormat := "[" + service + "] " + format
		log.Printf("DEBUG "+serviceFormat, v...)
	}
}

// SetSpanDefaults allows tuning span buffer size and flush tail length
func SetSpanDefaults(maxBuf, tail int) {
    if maxBuf > 0 { spanMaxBuf = maxBuf }
    if tail > 0 { spanTail = tail }
}

// ParseLevel converts a string to a Level. Defaults to LevelInfo.
func ParseLevel(s string) Level {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func SetLevel(l Level) { current = l }

func Debugf(format string, v ...interface{}) {
	if current <= LevelDebug {
		log.Printf("DEBUG "+format, v...)
	}
}

func Infof(format string, v ...interface{}) {
	if current <= LevelInfo {
		log.Printf(format, v...)
	}
}

func Warnf(format string, v ...interface{}) {
	if current <= LevelWarn {
		log.Printf("⚠️ "+format, v...)
	}
}

func Errorf(format string, v ...interface{}) {
	log.Printf("❌ "+format, v...)
}


func NewSpan(name string, kvs map[string]any) *Span {
	s := &Span{
		Name: name,
		KVs: kvs,
        buf: make([]Entry, 0, spanMaxBuf),
        maxBuf: spanMaxBuf,
		start: time.Now(),
	}
	return s
}

func (s *Span) add(l Level, msg string, kvs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := Entry{T: time.Now(), Level: l, Msg: msg, KVs: kvs}
	s.buf = append(s.buf, e)

	if len(s.buf) > s.maxBuf {
		s.buf = s.buf[len(s.buf) - s.maxBuf:]
	}

}


func (s *Span) Debug(msg string, kvs map[string]any) { s.add(LevelDebug, msg, kvs); Debugf("[%s] %s", s.Name, msg) }
func (s *Span) Info(msg string, kvs map[string]any)  { s.add(LevelInfo, msg, kvs);  Infof("[%s] %s", s.Name, msg) }
func (s *Span) Warn(msg string, kvs map[string]any)  { s.add(LevelWarn, msg, kvs);  Warnf("[%s] %s", s.Name, msg) }

func (s *Span) Error(msg string, kvs map[string]any) {
  s.add(LevelError, msg, kvs)
  s.flush("error")
}

func (s *Span) Finish(status string) {
  if status == "ok" && current > LevelDebug {
    // summary only
    Infof("✅ [%s] done in %dms", s.Name, time.Since(s.start).Milliseconds())
    return
  }
  s.flush(status)
}

func (s *Span) flush(status string) {
  s.mu.Lock()
  entries := append([]Entry(nil), s.buf...)
  s.mu.Unlock()

  log.Printf("─ span [%s] status=%s dur=%dms kvs=%v", s.Name, status, time.Since(s.start).Milliseconds(), s.KVs)
  // emit last N debug entries for context
  // Filter entries based on global level; drop DEBUG if current > LevelDebug
  filtered := make([]Entry, 0, len(entries))
  for _, e := range entries {
    if e.Level == LevelDebug && current > LevelDebug { continue }
    if e.Level == LevelInfo && current > LevelInfo { continue }
    if e.Level == LevelWarn && current > LevelWarn { continue }
    // errors always included
    filtered = append(filtered, e)
  }
  tail := filtered
  if len(tail) > spanTail { tail = tail[len(tail)-spanTail:] }
  for _, e := range tail {
    log.Printf("  %s %-5s %s kv=%v", e.T.Format("15:04:05.000"), levelString(e.Level), e.Msg, e.KVs)
  }
}

// Context helpers
type spanKeyT struct{}
var spanKey spanKeyT

func WithSpan(ctx context.Context, s *Span) context.Context { return context.WithValue(ctx, spanKey, s) }
func SpanFrom(ctx context.Context) *Span {
  if v := ctx.Value(spanKey); v != nil {
    if s, ok := v.(*Span); ok { return s }
  }
  return nil
}

func levelString(l Level) string { switch l { case LevelDebug: return "DEBUG"; case LevelInfo: return "INFO"; case LevelWarn: return "WARN"; default: return "ERROR" } }

func SetLevelFromEnv(env string) {
	SetLevel(ParseLevel(env))
}

// -----------------------------------------------------------------------------
// KV helpers for inline structured logs
// -----------------------------------------------------------------------------

func DebugKV(msg string, kv map[string]any) { if current <= LevelDebug { log.Printf("DEBUG %s kv=%v", msg, kv) } }
func InfoKV(msg string, kv map[string]any)  { if current <= LevelInfo  { log.Printf("%s kv=%v", msg, kv) } }
func WarnKV(msg string, kv map[string]any)  { if current <= LevelWarn  { log.Printf("⚠️ %s kv=%v", msg, kv) } }
func ErrorKV(msg string, kv map[string]any) { log.Printf("❌ %s kv=%v", msg, kv) }

// -----------------------------------------------------------------------------
// Span enrichment helpers
// -----------------------------------------------------------------------------

// With adds a single key/value to the span context
func (s *Span) With(k string, v any) *Span {
    s.mu.Lock()
    if s.KVs == nil { s.KVs = map[string]any{} }
    s.KVs[k] = v
    s.mu.Unlock()
    return s
}

// WithKVs adds multiple key/values to the span context
func (s *Span) WithKVs(kv map[string]any) *Span {
    s.mu.Lock()
    if s.KVs == nil { s.KVs = map[string]any{} }
    for k, v := range kv { s.KVs[k] = v }
    s.mu.Unlock()
    return s
}

// Child creates a new span inheriting this span's KVs with additional metadata
func (s *Span) Child(name string, kv map[string]any) *Span {
    child := NewSpan(name, nil)
    child.WithKVs(s.KVs)
    if kv != nil { child.WithKVs(kv) }
    return child
}


/* 
Emoji cheat sheet for logger messages (copy/paste) -- straight from ChatGPT

General
- ✅ success   - ❌ error     - ⚠️ warn     - ℹ️ info     - 🐞 debug
- ⏱️ timing    - 🔁 retry     - 🚫 deny     - 🧹 cleanup  - 🧭 context

Startup & Lifecycle
- 🚀 startup   - 🙌 ready     - 🧠 LLM init - 🔊 STT init - 🗄️ DB init
- 🔐 auth init - ⚙️ config    - 🧰 tools    - 🧪 warmup   - 🧯 shutdown

WebSocket
- 🔌 connect   - 📥 reader    - 📤 writer   - 📴 close    - 🧵 session

Dynamic Config
- 🔄 update    - 🧩 schema    - 🧮 checksum - 🧼 reset prev fns - 🧰 draft idx

Cache (LLM)
- 🧱 cache     - 🗃️ store     - 🏷️ key     - 🗑️ invalidate - 🔍 validate
- ✅ hit       - ➖ miss       - ⤴️ skip     - 📦 size      - ✂️ saved

LLM
- 💬 request   - 🤖 model     - 🧰 tools    - 🧭 system   - 🧪 prompt
- 🧾 usage     - 📊 tokens    - 🧮 saved    - 📚 context  - 🧠 function-call

STT
- 🎙️ audio     - 🔈 chunk     - 📝 transcript - ✅ final   - 🧩 draft

Pipeline
- 🧵 pipeline  - 📨 emit      - 🛠️ merge    - 🧪 filter  - 📦 batch

Database
- 🗄️ query     - 🧾 upsert    - 🧷 id       - 🔒 tx       - 📁 repo

HTTP/API
- 🌐 request   - 🧭 route     - 🛡️ CORS     - 🔑 auth    - 🚫 401/403

Metrics/Observability
- 📈 metric    - 🧩 labels    - 🔎 inspect  - 🧭 trace

Performance/Resources
- ⚡ fast      - 🐢 slow      - 🖥️ cpu      - 🧠 mem     - 📡 network
- 📦 queue     - 🔥 hot path

Controls
- ▶️ start     - ⏸️ pause     - ⏹️ stop     - 🔁 retry   - 🔂 loop

Notes/Docs
- 📝 note      - 🧪 test      - 📚 ref      - 📌 todo
*/