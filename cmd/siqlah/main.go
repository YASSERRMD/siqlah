// Command siqlah is the main siqlah ledger service.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yasserrmd/siqlah/internal/anchor"
	"github.com/yasserrmd/siqlah/internal/api"
	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/monitor"
	"github.com/yasserrmd/siqlah/internal/provider"
	"github.com/yasserrmd/siqlah/internal/signing"
	"github.com/yasserrmd/siqlah/internal/store"
)

var (
	// Injected at build time via -ldflags.
	version   = "dev"
	commitSHA = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate(os.Args[2:])
		return
	}

	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "siqlah.db", "SQLite database path")
	batchInterval := flag.Duration("batch-interval", 30*time.Second, "interval between automatic checkpoint builds")
	maxBatch := flag.Int("max-batch", 1000, "max receipts per checkpoint")
	witnessesFlag := flag.String("witnesses", "", "comma-separated witness_id=pubhex pairs")
	enableMonitor := flag.Bool("monitor", false, "enable discrepancy monitor")
	monitorInterval := flag.Duration("monitor-interval", 60*time.Second, "monitor polling interval")
	discThreshold := flag.Float64("discrepancy-threshold", 5.0, "discrepancy alert threshold percent")
	alertWebhook := flag.String("alert-webhook", "", "webhook URL for discrepancy alerts")
	operatorKeyHex := flag.String("operator-key", "", "hex-encoded Ed25519 private key (generated if empty)")
	logBackend := flag.String("log-backend", "sqlite", "log backend: sqlite (legacy) or tessera")
	tesseraPath := flag.String("tessera-storage-path", "./tessera-data/", "POSIX path for Tessera tile storage")
	tesseraLogName := flag.String("tessera-log-name", "siqlah.dev/log", "C2SP log origin string for Tessera")
	signingBackend := flag.String("signing-backend", "ed25519", "signing backend: ed25519 or fulcio")
	oidcIssuer := flag.String("oidc-issuer", "https://accounts.google.com", "OIDC issuer URL for Fulcio keyless signing")
	oidcClientID := flag.String("oidc-client-id", "", "OIDC client ID for Fulcio token flow")
	rekorURL := flag.String("rekor-url", "", "Rekor transparency log URL (empty disables Rekor logging)")
	fulcioURL := flag.String("fulcio-url", "https://fulcio.sigstore.dev", "Fulcio CA endpoint for keyless signing")
	rekorAnchor := flag.Bool("rekor-anchor", false, "enable periodic Rekor public anchoring")
	rekorInterval := flag.Duration("rekor-anchor-interval", 24*time.Hour, "interval between Rekor anchoring attempts")
	inferenceRegion := flag.String("inference-region", "", "cloud region where inference runs (e.g. us-east-1) for carbon reporting")
	x402Recipient := flag.String("x402-recipient", "", "EVM address to receive x402 payments (empty disables payment routing)")
	flag.Parse()

	_, _ = *oidcClientID, *oidcIssuer // surfaced for future integration

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	printBanner()

	// Load or generate the operator keypair.
	var operatorPriv ed25519.PrivateKey
	var operatorPub ed25519.PublicKey
	if *operatorKeyHex != "" {
		privBytes, err := hex.DecodeString(*operatorKeyHex)
		if err != nil || len(privBytes) != ed25519.PrivateKeySize {
			slog.Error("invalid --operator-key", "expected_bytes", ed25519.PrivateKeySize)
			os.Exit(1)
		}
		operatorPriv = ed25519.PrivateKey(privBytes)
		operatorPub = operatorPriv.Public().(ed25519.PublicKey)
	} else {
		var err error
		operatorPub, operatorPriv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			slog.Error("generate operator key", "error", err)
			os.Exit(1)
		}
		slog.Warn("generated ephemeral operator key — set --operator-key to persist across restarts")
	}
	slog.Info("operator key loaded", "public_key", hex.EncodeToString(operatorPub))

	// Wire the signing backend.
	var receiptSigner signing.Signer
	switch *signingBackend {
	case "fulcio":
		slog.Info("signing backend: fulcio (keyless)", "oidc_issuer", *oidcIssuer)
		receiptSigner = signing.NewFulcioSigner(signing.FulcioOptions{
			FulcioURL: *fulcioURL,
			RekorURL:  *rekorURL,
		})
	default:
		slog.Info("signing backend: ed25519")
		receiptSigner = signing.NewEd25519Signer(operatorPriv)
	}
	_ = receiptSigner // passed to API in future integration

	// Open the store — SQLite (legacy) or Tessera (production).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var st store.Store
	var builder interface{ BuildAndSign() (*store.Checkpoint, error) }

	if *logBackend == "tessera" {
		slog.Info("using Tessera backend", "path", *tesseraPath)
		ts, err := store.NewTesseraStore(ctx, *dbPath, *tesseraPath, *tesseraLogName, operatorPriv)
		if err != nil {
			slog.Error("open tessera store", "error", err)
			os.Exit(1)
		}
		st = ts
		builder = checkpoint.NewTesseraBuilder(st, operatorPriv, *maxBatch)
	} else {
		sqlite, err := store.Open(*dbPath)
		if err != nil {
			slog.Error("open store", "error", err)
			os.Exit(1)
		}
		st = sqlite
		builder = checkpoint.NewBuilder(st, operatorPriv, *maxBatch)
	}
	defer st.Close()

	// Parse witness public keys.
	_ = parseWitnesses(*witnessesFlag)

	// Build the API server. The builder is type-asserted to *checkpoint.Builder for the API
	// (which only needs BuildAndSign for the manual trigger endpoint).
	var cpBuilder *checkpoint.Builder
	if b, ok := builder.(*checkpoint.Builder); ok {
		cpBuilder = b
	}
	reg := provider.NewRegistry()
	srv := api.NewWithOptions(st, cpBuilder, operatorPub, operatorPriv, reg, version, *inferenceRegion)
	srv.WithBackendInfo(*logBackend, *signingBackend)
	if *x402Recipient != "" {
		srv.WithX402Recipient(*x402Recipient)
	}

	// Start periodic batcher.
	go func() {
		ticker := time.NewTicker(*batchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cp, err := builder.BuildAndSign()
				if err != nil {
					slog.Error("batcher: build checkpoint failed", "error", err)
				} else if cp != nil {
					slog.Info("batcher: checkpoint built", "id", cp.ID, "tree_size", cp.TreeSize)
				}
			}
		}
	}()

	// Optionally start Rekor public anchoring scheduler.
	if *rekorAnchor {
		ra, err := anchor.NewRekorAnchor(*rekorURL)
		if err != nil {
			slog.Error("create rekor anchor", "error", err)
			os.Exit(1)
		}
		sched := anchor.NewAnchorScheduler(ra, st, *rekorInterval)
		go sched.Run(ctx)
		slog.Info("rekor anchoring enabled", "url", *rekorURL, "interval", *rekorInterval)
	}

	// Optionally start discrepancy monitor.
	if *enableMonitor {
		var alerter monitor.Alerter = &monitor.LogAlerter{}
		if *alertWebhook != "" {
			alerter = monitor.NewMultiAlerter(&monitor.LogAlerter{}, monitor.NewWebhookAlerter(*alertWebhook))
		}
		mon := monitor.New(monitor.Config{
			LedgerURL: fmt.Sprintf("http://localhost%s", *addr),
			Alerter:   alerter,
			Interval:  *monitorInterval,
			Threshold: *discThreshold,
		})
		go mon.Run(ctx)
	}

	// Set up HTTP server.
	httpSrv := &http.Server{
		Addr:         *addr,
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Listen for shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down...")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			slog.Error("graceful shutdown error", "error", err)
		}
	}()

	slog.Info("siqlah listening", "version", version, "commit", commitSHA, "addr", *addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("listen failed", "error", err)
		os.Exit(1)
	}
}

// parseWitnesses parses a comma-separated "id=pubhex" string into a map.
func parseWitnesses(s string) map[string]ed25519.PublicKey {
	out := map[string]ed25519.PublicKey{}
	if s == "" {
		return out
	}
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			slog.Warn("invalid witness pair, expected id=pubhex", "pair", pair)
			continue
		}
		b, err := hex.DecodeString(parts[1])
		if err != nil || len(b) != ed25519.PublicKeySize {
			slog.Warn("invalid witness pubkey", "id", parts[0])
			continue
		}
		out[parts[0]] = ed25519.PublicKey(b)
	}
	return out
}

func printBanner() {
	fmt.Println(`
  ███████╗██╗ ██████╗ ██╗      █████╗ ██╗  ██╗
  ██╔════╝██║██╔═══██╗██║     ██╔══██╗██║  ██║
  ███████╗██║██║   ██║██║     ███████║███████║
  ╚════██║██║██║▄▄ ██║██║     ██╔══██║██╔══██║
  ███████║██║╚██████╔╝███████╗██║  ██║██║  ██║
  ╚══════╝╚═╝ ╚══▀▀═╝ ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝
  Verifiable Usage Receipt ledger — سِقلة`)
}
