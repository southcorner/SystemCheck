// Command server runs the SystemCheck ingestion API and admin backend.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/southcorner/systemcheck/server/internal/api"
	"github.com/southcorner/systemcheck/server/internal/auth"
	"github.com/southcorner/systemcheck/server/internal/blob"
	"github.com/southcorner/systemcheck/server/internal/config"
	"github.com/southcorner/systemcheck/server/internal/model"
	"github.com/southcorner/systemcheck/server/internal/pki"
	"github.com/southcorner/systemcheck/server/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("systemcheck ")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	// Dev convenience: generate self-signed CA + server cert if absent.
	if err := pki.EnsureDevCerts(cfg.CACert, cfg.CAKey, cfg.TLSCert, cfg.TLSKey); err != nil {
		log.Fatalf("dev certs: %v", err)
	}

	st, err := store.Open(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	bootstrapAdmin(ctx, st, cfg)

	bl, err := blob.NewFSStore("./data/screenshots", cfg.ScreenshotKey)
	if err != nil {
		log.Fatalf("blob: %v", err)
	}

	ca, err := pki.LoadCA(cfg.CACert, cfg.CAKey)
	if err != nil {
		log.Fatalf("load ca: %v", err)
	}

	app := api.New(cfg, st, bl, ca)
	tlsCfg, err := app.TLSConfig()
	if err != nil {
		log.Fatalf("tls: %v", err)
	}

	go runPurgeLoop(ctx, st, bl, cfg)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Router(),
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (https, mTLS for agents)", cfg.ListenAddr)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

// runPurgeLoop enforces retention windows once at startup and then daily.
func runPurgeLoop(ctx context.Context, st *store.Store, bl *blob.FSStore, cfg *config.Config) {
	purge := func() {
		cutoffs := store.RetentionCutoffs(time.Now(),
			cfg.RetentionScreenshotsDays, cfg.RetentionActivityDays,
			cfg.RetentionSecurityDays, cfg.RetentionAuditDays)
		res, err := st.Purge(ctx, cutoffs, bl.Delete)
		if err != nil {
			log.Printf("purge error: %v", err)
			return
		}
		log.Printf("retention purge: events=%d screenshots=%d app_usage=%d audit=%d",
			res.Events, res.Screenshots, res.AppUsage, res.Audit)
	}
	purge()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purge()
		}
	}
}

// bootstrapAdmin creates the first admin from env if no admins exist yet.
func bootstrapAdmin(ctx context.Context, st *store.Store, cfg *config.Config) {
	n, err := st.CountAdmins(ctx)
	if err != nil {
		log.Fatalf("count admins: %v", err)
	}
	if n > 0 {
		return
	}
	if cfg.BootstrapAdminEmail == "" || cfg.BootstrapAdminPassword == "" {
		log.Println("no admins and no bootstrap credentials set; set SC_BOOTSTRAP_ADMIN_* to create the first admin")
		return
	}
	hash, err := auth.HashPassword(cfg.BootstrapAdminPassword)
	if err != nil {
		log.Fatalf("hash bootstrap password: %v", err)
	}
	if _, err := st.CreateAdmin(ctx, cfg.BootstrapAdminEmail, hash, model.RoleAdmin); err != nil {
		log.Fatalf("create bootstrap admin: %v", err)
	}
	log.Printf("created bootstrap admin %s (change the password and enroll MFA on first login)", cfg.BootstrapAdminEmail)
}
