package guardian

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	mathrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/internal/server"
	"github.com/fulmenhq/goneat/pkg/ascii"
	"github.com/fulmenhq/goneat/pkg/buildinfo"
	"github.com/fulmenhq/goneat/pkg/logger"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const instructionBoxWidth = 76

// ApprovalSession describes the approval request context used by the browser server.
type ApprovalSession struct {
	Scope         string
	Operation     string
	Policy        *ResolvedPolicy
	Reason        string
	Context       string
	RequestedAt   time.Time
	Expires       time.Time
	ProjectName   string
	CustomMessage string
}

// BrowserServer manages the lifecycle of the guardian approval server.
type BrowserServer struct {
	info            server.Info
	httpServer      *http.Server
	listener        net.Listener
	nonce           string
	session         ApprovalSession
	started         time.Time
	done            chan error
	once            sync.Once
	expires         time.Time
	effectiveExpiry time.Duration
	projectName     string
	projectFolder   string
	customMessage   string
	showURL         bool
	expireTimer     *time.Timer
	mu              sync.Mutex
	completed       bool
	expired         bool
	expireErr       error
	cleanupOnce     sync.Once
}

// ErrApprovalExpired indicates the approval session expired before completion.
var ErrApprovalExpired = errors.New("guardian approval expired")

func getMachineName() string {
	hostname, _ := os.Hostname()
	return hostname
}

func getProjectFolder() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	if output, err := cmd.Output(); err == nil {
		repoPath := strings.TrimSpace(string(output))
		if repoName := filepath.Base(repoPath); repoName != "" && repoName != "." {
			return repoName
		}
	}
	pwd, _ := os.Getwd()
	return filepath.Base(pwd)
}

// StartBrowserApproval launches the guardian browser approval server.
func StartBrowserApproval(ctx context.Context, session ApprovalSession) (*BrowserServer, error) {
	if session.Policy == nil {
		return nil, errors.New("approval session missing policy")
	}
	if session.Scope == "" || session.Operation == "" {
		return nil, errors.New("approval session missing scope or operation")
	}

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	browserCfg := cfg.Guardian.Security.Browser
	branding := cfg.Guardian.Security.Branding

	projectFolder := getProjectFolder()
	projectName := strings.TrimSpace(branding.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(session.ProjectName)
	}
	if projectName == "" {
		projectName = projectFolder
	}
	session.ProjectName = projectName
	session.CustomMessage = strings.TrimSpace(branding.CustomMessage)

	port, listener, err := allocateListener(browserCfg)
	if err != nil {
		return nil, err
	}

	nonce, err := generateNonce()
	if err != nil {
		_ = listener.Close() // Best effort cleanup
		return nil, err
	}

	effectiveExpiry := session.Policy.Expires
	if browserCfg.TimeoutSeconds > 0 {
		timeout := time.Duration(browserCfg.TimeoutSeconds) * time.Second
		if effectiveExpiry == 0 || timeout < effectiveExpiry {
			effectiveExpiry = timeout
		}
	}
	if effectiveExpiry <= 0 {
		effectiveExpiry = 5 * time.Minute
	}
	expires := time.Now().UTC().Add(effectiveExpiry)
	session.Expires = expires

	srv := &BrowserServer{
		nonce:           nonce,
		listener:        listener,
		started:         time.Now().UTC(),
		done:            make(chan error, 1),
		session:         session,
		expires:         expires,
		effectiveExpiry: effectiveExpiry,
		projectName:     projectName,
		projectFolder:   projectFolder,
		customMessage:   session.CustomMessage,
		showURL:         browserCfg.ShowURL,
	}

	srv.info = server.Info{
		Name:      "guardian",
		Port:      port,
		PID:       os.Getpid(),
		Version:   buildinfo.BinaryVersion,
		StartedAt: srv.started,
		Metadata: map[string]string{
			"scope":     session.Scope,
			"operation": session.Operation,
			"method":    string(session.Policy.Method),
			"project":   projectFolder,
			"machine":   getMachineName(),
		},
	}

	if err := server.Save(srv.info); err != nil {
		_ = listener.Close() // Best effort cleanup
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", srv.handleHello)
	mux.HandleFunc("/", srv.handleRedirect)
	mux.HandleFunc("/approve/", srv.handleApproval)

	srv.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go srv.serve()
	go srv.watchContext(ctx)

	if shouldAutoOpen(browserCfg) {
		go srv.tryOpenBrowser()
	}

	if err := srv.displayApprovalInstructions(srv.ApprovalURL()); err != nil {
		logger.Warn("Failed to display approval instructions", logger.Err(err))
	}

	if srv.effectiveExpiry > 0 {
		srv.expireTimer = time.NewTimer(srv.effectiveExpiry)
		go srv.monitorExpiry()
	}

	return srv, nil
}

// URL returns the base URL for the approval server.
func (b *BrowserServer) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", b.info.Port)
}

// ApprovalURL returns the approval URL that includes the nonce token.
func (b *BrowserServer) ApprovalURL() string {
	return fmt.Sprintf("%s/approve/%s", b.URL(), b.nonce)
}

// Wait blocks until the server stops and cleans up the metadata entry.
func (b *BrowserServer) Wait() error {
	err := <-b.done
	b.mu.Lock()
	b.completed = true
	if b.expireTimer != nil {
		_ = b.expireTimer.Stop()
	}
	expireErr := b.expireErr
	b.mu.Unlock()
	b.cleanup()
	if expireErr != nil {
		return expireErr
	}
	return normalizeServerError(err)
}

// Shutdown stops the server proactively.
func (b *BrowserServer) Shutdown(ctx context.Context) error {
	b.once.Do(func() {
		if b.httpServer != nil {
			_ = b.httpServer.Shutdown(ctx)
		}
	})
	return b.Wait()
}

func (b *BrowserServer) serve() {
	err := b.httpServer.Serve(b.listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Warn("Guardian browser server ended unexpectedly", logger.Err(err))
	}
	b.done <- err
	close(b.done)
}

func (b *BrowserServer) watchContext(ctx context.Context) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b.once.Do(func() {
		if b.httpServer != nil {
			_ = b.httpServer.Shutdown(shutdownCtx)
		}
	})
}

func (b *BrowserServer) cleanup() {
	b.cleanupOnce.Do(func() {
		_ = server.Remove(b.info.Name)
		if b.listener != nil {
			_ = b.listener.Close()
		}
	})
}

func (b *BrowserServer) handleHello(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(server.HelloResponse{
		Name:      "guardian",
		Version:   buildinfo.BinaryVersion,
		StartedAt: b.started,
	})
}

func (b *BrowserServer) handleRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, b.ApprovalURL(), http.StatusTemporaryRedirect)
}

func (b *BrowserServer) tryOpenBrowser() {
	cmd, err := browserCommand(b.ApprovalURL())
	if err != nil {
		logger.Debug("Guardian browser auto-open skipped", logger.Err(err))
		return
	}
	if err := cmd.Start(); err != nil {
		logger.Debug("Guardian browser auto-open failed", logger.Err(err))
	}
}

// handleApproval handles all /approve/ requests (GET for page, POST for actions).
func (b *BrowserServer) handleApproval(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "approve" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// GET /approve/{nonce} - render page
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		nonce := parts[1]
		if nonce != b.nonce {
			http.NotFound(w, r)
			return
		}

		tmplData := &approvalPage{
			ApprovalSession:  b.session,
			ProjectName:      b.projectName,
			CustomMessage:    b.customMessage,
			Nonce:            b.nonce,
			Started:          b.started,
			Expires:          b.expires,
			MachineName:      getMachineName(),
			ProjectFolder:    b.projectFolder,
			RiskLevel:        b.session.Policy.Risk,
			ExpiresIn:        formatTimeUntil(b.expires),
			ExpiresInSeconds: int(time.Until(b.expires).Seconds()),
			Timestamp:        b.session.RequestedAt.Format(time.RFC3339),
		}

		tmpl, err := tmplData.template()
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, tmplData); err != nil {
			logger.Debug("Guardian approval template execution failed", logger.Err(err))
		}
	case http.MethodPost:
		// POST /approve/{nonce}/{confirm|deny} - handle action
		if len(parts) < 3 {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		nonce := parts[1]
		action := parts[2]

		if nonce != b.nonce {
			http.Error(w, "invalid nonce", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch action {
		case "confirm":
			logger.Info("Guardian approval confirmed via browser", logger.String("scope", b.session.Scope), logger.String("operation", b.session.Operation))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "approved", "message": "Operation approved - returning to terminal"})
			// Signal success and shutdown
			b.done <- nil
			go func() {
				time.Sleep(1 * time.Second)
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = b.Shutdown(shutdownCtx)
			}()
		case "deny":
			logger.Info("Guardian approval denied via browser", logger.String("scope", b.session.Scope), logger.String("operation", b.session.Operation))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "denied", "message": "Operation denied - check terminal for details"})
			// Signal denial and shutdown
			b.done <- errors.New("operation denied by user")
			go func() {
				time.Sleep(1 * time.Second)
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = b.Shutdown(shutdownCtx)
			}()
		default:
			http.Error(w, "invalid action (confirm or deny)", http.StatusBadRequest)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func formatTimeUntil(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return "expired"
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}

// approvalPage is the template renderer (now private since handled in BrowserServer).
type approvalPage struct {
	ApprovalSession
	ProjectName      string
	CustomMessage    string
	Nonce            string
	Started          time.Time
	Expires          time.Time
	MachineName      string
	ProjectFolder    string
	RiskLevel        string
	ExpiresIn        string
	ExpiresInSeconds int
	Timestamp        string
	compiled         *template.Template
	compileMu        sync.Mutex
}

const fallbackTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{.ProjectName}} - Guardian Approval</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; color: #1a202c; }
    h1 { margin-bottom: 0.25rem; }
    h2 { margin-top: 0; color: #4a5568; }
    .card { border: 1px solid #cbd5f5; padding: 1.5rem; border-radius: 0.75rem; max-width: 540px; }
    .info { background: #edf2f7; padding: 1rem; border-radius: 0.5rem; margin: 1rem 0; }
    button { padding: 0.75rem 1rem; border: none; border-radius: 0.5rem; margin-right: 0.5rem; cursor: pointer; font-weight: 600; }
    .approve { background: #38a169; color: #fff; }
    .deny { background: #e53e3e; color: #fff; }
  </style>
</head>
<body>
  <div class="card">
    <h1>{{.ProjectName}}</h1>
    <h2>Guardian approval required</h2>
    {{if .CustomMessage}}<p>{{.CustomMessage}}</p>{{end}}
    <div class="info">
      <p><strong>Operation:</strong> {{.Scope}}.{{.Operation}}</p>
      <p><strong>Risk:</strong> {{.RiskLevel}}</p>
      <p><strong>Requested:</strong> {{.Timestamp}}</p>
      <p><strong>From:</strong> {{.MachineName}} / {{.ProjectFolder}}</p>
      <p><strong>Expires in:</strong> {{.ExpiresIn}}</p>
    </div>
    <button class="approve" onclick="approve()">Approve</button>
    <button class="deny" onclick="deny()">Deny</button>
  </div>
  <script>
  async function approve() {
    await fetch('/approve/{{.Nonce}}/confirm', { method: 'POST', headers: {'Content-Type': 'application/json'} });
    document.body.innerHTML = '<h2>✅ Approved</h2><p>You may close this window.</p>';
  }
  async function deny() {
    await fetch('/approve/{{.Nonce}}/deny', { method: 'POST', headers: {'Content-Type': 'application/json'} });
    document.body.innerHTML = '<h2>❌ Denied</h2><p>You may close this window.</p>';
  }
  </script>
</body>
</html>`

func (a *approvalPage) template() (*template.Template, error) {
	a.compileMu.Lock()
	defer a.compileMu.Unlock()

	if a.compiled != nil {
		return a.compiled, nil
	}

	funcMap := template.FuncMap{
		"title": cases.Title(language.English).String,
		"substr": func(s string, start int) string {
			if len(s) > start+8 {
				return s[:start+8]
			}
			return s
		},
		"formatTimeUntil": func(t time.Time) string {
			d := time.Until(t)
			if d < 0 {
				return "expired"
			}
			mins := int(d.Minutes())
			secs := int(d.Seconds()) % 60
			return fmt.Sprintf("%d:%02d", mins, secs)
		},
	}

	// Try embedded template first
	var tmpl *template.Template
	data, err := fs.ReadFile(assets.Templates, "embedded_templates/templates/guardian/approval.html")
	if err != nil {
		// Fallback to const template
		logger.Warn("Embedded template failed, using fallback", logger.Err(err))
		tmpl, err = template.New("guardian-approval").Funcs(funcMap).Parse(fallbackTemplate)
		if err != nil {
			return nil, err
		}
	} else {
		tmpl, err = template.New("guardian-approval").Funcs(funcMap).Parse(string(data))
		if err != nil {
			return nil, err
		}
	}
	a.compiled = tmpl
	return tmpl, nil
}

func (b *BrowserServer) displayApprovalInstructions(url string) error {
	if !b.showURL {
		fmt.Println("\n❌ Operation blocked by guardian")
		fmt.Printf("🔐 Approval required for: %s.%s\n", b.session.Scope, b.session.Operation)
		fmt.Printf("🛡️  Project: %s\n", b.projectName)
		if b.session.Reason != "" {
			fmt.Printf("   Reason: %s\n", b.session.Reason)
		}
		fmt.Println("\nGuardian approval server is running locally.")
		fmt.Println("Follow the instructions provided via your configured approval channel.")
		return nil
	}

	timeLeft := time.Until(b.expires)
	if timeLeft < 0 {
		timeLeft = 0
	}
	minutes := int(timeLeft.Minutes())
	seconds := int(timeLeft.Seconds()) % 60
	machineName := getMachineName()

	fmt.Println("\n❌ Operation blocked by guardian")
	fmt.Printf("🔐 Approval required for: %s.%s\n", b.session.Scope, b.session.Operation)
	fmt.Printf("🛡️  Project: %s\n", b.projectName)
	if b.session.Reason != "" {
		fmt.Printf("   Reason: %s\n", b.session.Reason)
	}
	fmt.Println()

	trim := func(msg string) string {
		return ascii.TruncateForBox(msg, instructionBoxWidth)
	}

	// Create title bar with separator
	titleLine := trim(fmt.Sprintf("GUARDIAN APPROVAL REQUIRED for project %s on operation '%s.%s'", b.projectName, b.session.Scope, b.session.Operation))
	separator := strings.Repeat("═", ascii.StringWidth(titleLine)) // Match title display width using ASCII library

	lines := []string{
		titleLine,
		separator,
		"",
		trim("Open this URL in your browser to approve/deny the operation:"),
		"",
		trim(fmt.Sprintf("🔗  %s", url)),
		"",
		trim(fmt.Sprintf("⏱️  Expires in: %2d:%02d", minutes, seconds)),
		"",
		trim("📋  Copy the URL: Select the link above or use Ctrl+C / right-click copy"),
		"",
		trim(fmt.Sprintf("📂  Project folder: %s", b.projectFolder)),
		trim(fmt.Sprintf("💻  Machine: %s", machineName)),
	}

	if b.customMessage != "" {
		lines = append(lines, "")
		lines = append(lines, trim(fmt.Sprintf("💬  %s", b.customMessage)))
	}

	lines = append(lines, "")
	lines = append(lines, trim("ℹ️  Auto-open was attempted (if enabled). If it opened in the wrong"))
	lines = append(lines, trim("     browser/profile, or this is CI/CD/headless, paste the URL manually."))
	lines = append(lines, trim("     No browser? Use curl or another tool to visit the URL."))

	ascii.DrawBox(lines)
	fmt.Println("\n⏳ Waiting for approval... (Ctrl+C to cancel)")

	return nil
}

func (b *BrowserServer) monitorExpiry() {
	if b.expireTimer == nil {
		return
	}
	<-b.expireTimer.C
	b.mu.Lock()
	if b.completed {
		b.mu.Unlock()
		return
	}
	b.expired = true
	b.expireErr = ErrApprovalExpired
	b.mu.Unlock()

	logger.Info("Guardian approval expired", logger.String("scope", b.session.Scope), logger.String("operation", b.session.Operation))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b.once.Do(func() {
		if b.httpServer != nil {
			_ = b.httpServer.Shutdown(shutdownCtx)
		}
	})
}

func allocateListener(cfg BrowserSettings) (int, net.Listener, error) {
	minPort, maxPort := server.PortMin, server.PortMax
	if len(cfg.PortRange) == 2 {
		minPort, maxPort = cfg.PortRange[0], cfg.PortRange[1]
	}
	if minPort <= 0 {
		minPort = server.PortMin
	}
	if maxPort <= 0 || maxPort < minPort {
		maxPort = server.PortMax
	}

	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano())) // #nosec G404 - Port allocation doesn't require cryptographic randomness
	for i := 0; i < 25; i++ {
		candidate := rng.Intn(maxPort-minPort+1) + minPort
		addr := fmt.Sprintf("127.0.0.1:%d", candidate)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return candidate, listener, nil
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, fmt.Errorf("failed to allocate port for guardian browser server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	return port, listener, nil
}

func generateNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func shouldAutoOpen(cfg BrowserSettings) bool {
	if val := strings.TrimSpace(os.Getenv("GONEAT_GUARDIAN_AUTO_OPEN")); val != "" {
		return val == "1" || strings.EqualFold(val, "true")
	}
	return cfg.AutoOpen
}

func browserCommand(url string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url), nil
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url), nil
	case "linux":
		return exec.Command("xdg-open", url), nil
	default:
		return nil, fmt.Errorf("unsupported platform for browser auto-open")
	}
}

func normalizeServerError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// EffectiveExpiry returns the duration the server will wait before expiring.
func (b *BrowserServer) EffectiveExpiry() time.Duration {
	return b.effectiveExpiry
}

// ExpiresAt returns the timestamp when the approval session expires.
func (b *BrowserServer) ExpiresAt() time.Time {
	return b.expires
}
