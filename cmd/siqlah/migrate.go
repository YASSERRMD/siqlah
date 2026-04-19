package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yasserrmd/siqlah/internal/store"
)

// runMigrate implements the "migrate" subcommand: copies all receipts from a
// SQLite source database into a new Tessera-backed destination store.
//
// Usage:
//
//	siqlah migrate --src=old.db --tessera-storage-path=./tessera-data/
func runMigrate(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	src := fs.String("src", "siqlah.db", "source SQLite database path")
	dst := fs.String("dst", "siqlah-v2.db", "destination SQLite database path (used alongside Tessera)")
	tesseraPath := fs.String("tessera-storage-path", "./tessera-data/", "POSIX path for Tessera tile storage")
	tesseraLogName := fs.String("tessera-log-name", "siqlah.dev/log", "C2SP log origin string for Tessera")
	operatorKeyHex := fs.String("operator-key", "", "hex-encoded Ed25519 private key for Tessera signer")
	batchSize := fs.Int("batch-size", 500, "receipts processed per batch")
	dryRun := fs.Bool("dry-run", false, "scan source and print count without writing to destination")
	_ = fs.Parse(args)

	if *operatorKeyHex == "" && !*dryRun {
		fmt.Fprintln(os.Stderr, "error: --operator-key is required for non-dry-run migration")
		fs.Usage()
		os.Exit(1)
	}

	srcStore, err := store.Open(*src)
	if err != nil {
		log.Fatalf("open source: %v", err)
	}
	defer srcStore.Close()

	stats, err := srcStore.Stats()
	if err != nil {
		log.Fatalf("source stats: %v", err)
	}
	log.Printf("source: %d receipts to migrate", stats.TotalReceipts)

	if *dryRun {
		fmt.Printf("dry-run: would migrate %d receipts from %q to Tessera at %q\n",
			stats.TotalReceipts, *src, *tesseraPath)
		return
	}

	privBytes, err := decodeHexKey(*operatorKeyHex)
	if err != nil {
		log.Fatalf("invalid --operator-key: %v", err)
	}

	ctx := context.Background()
	dstStore, err := store.NewTesseraStore(ctx, *dst, *tesseraPath, *tesseraLogName, ed25519.PrivateKey(privBytes))
	if err != nil {
		log.Fatalf("open destination: %v", err)
	}
	defer dstStore.Close()

	var (
		startID  int64 = 1
		migrated int64
	)
	for {
		endID := startID + int64(*batchSize) - 1
		batch, err := srcStore.GetReceiptsByRange(startID, endID)
		if err != nil {
			log.Fatalf("read batch starting at %d: %v", startID, err)
		}
		if len(batch) == 0 {
			break
		}
		for _, r := range batch {
			if _, err := dstStore.AppendReceipt(r); err != nil {
				log.Fatalf("append receipt %s: %v", r.ID, err)
			}
			migrated++
		}
		log.Printf("migrated %d/%d receipts...", migrated, stats.TotalReceipts)
		startID = endID + 1
	}
	log.Printf("migration complete: %d receipts written to Tessera store", migrated)
}

func decodeHexKey(s string) ([]byte, error) {
	if len(s) != ed25519.PrivateKeySize*2 {
		return nil, fmt.Errorf("must be %d-byte hex string", ed25519.PrivateKeySize)
	}
	b := make([]byte, ed25519.PrivateKeySize)
	for i := 0; i < len(s); i += 2 {
		var v byte
		_, err := fmt.Sscanf(s[i:i+2], "%02x", &v)
		if err != nil {
			return nil, fmt.Errorf("invalid hex at position %d: %w", i, err)
		}
		b[i/2] = v
	}
	return b, nil
}
