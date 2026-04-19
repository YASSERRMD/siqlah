// Command siqlah is the main siqlah ledger service.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yasserrmd/siqlah/internal/api"
	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/monitor"
	"github.com/yasserrmd/siqlah/internal/provider"
	"github.com/yasserrmd/siqlah/internal/store"
)

var (
	// Injected at build time via -ldflags.
	version   = "dev"
	commitSHA = "unknown"
)

func main() {
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
	flag.Parse()

	printBanner()

	// Load or generate the operator keypair.
	var operatorPriv ed25519.PrivateKey
	var operatorPub ed25519.PublicKey
	if *operatorKeyHex != "" {
		privBytes, err := hex.DecodeString(*operatorKeyHex)
		if err != nil || len(privBytes) != ed25519.PrivateKeySize {
			log.Fatalf("invalid --operator-key: must be %d-byte hex", ed25519.PrivateKeySize)
		}
		operatorPriv = ed25519.PrivateKey(privBytes)
		operatorPub = operatorPriv.Public().(ed25519.PublicKey)
	} else {
		var err error
		operatorPub, operatorPriv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatalf("generate operator key: %v", err)
		}
		log.Printf("generated operator key (ephemeral — set --operator-key to persist)")
	}
	log.Printf("operator public key: %s", hex.EncodeToString(operatorPub))

	// Open the store — SQLite (legacy) or Tessera (production).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var st store.Store
	var builder interface{ BuildAndSign() (*store.Checkpoint, error) }

	if *logBackend == "tessera" {
		log.Printf("using Tessera backend at %s", *tesseraPath)
		ts, err := store.NewTesseraStore(ctx, *dbPath, *tesseraPath, *tesseraLogName, operatorPriv)
		if err != nil {
			log.Fatalf("open tessera store: %v", err)
		}
		st = ts
		builder = checkpoint.NewTesseraBuilder(st, operatorPriv, *maxBatch)
	} else {
		sqlite, err := store.Open(*dbPath)
		if err != nil {
			log.Fatalf("open store: %v", err)
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
	srv := api.New(st, cpBuilder, operatorPub, operatorPriv, reg, version)

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
					log.Printf("batcher: build checkpoint: %v", err)
				} else if cp != nil {
					log.Printf("batcher: checkpoint %d built (tree_size=%d)", cp.ID, cp.TreeSize)
				}
			}
		}
	}()

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
		log.Println("shutting down...")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("siqlah %s (%s) listening on %s", version, commitSHA, *addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
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
			log.Printf("warning: invalid witness pair %q, expected id=pubhex", pair)
			continue
		}
		b, err := hex.DecodeString(parts[1])
		if err != nil || len(b) != ed25519.PublicKeySize {
			log.Printf("warning: invalid witness pubkey for %q", parts[0])
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
