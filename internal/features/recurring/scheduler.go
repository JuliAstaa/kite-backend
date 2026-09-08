package recurring

import (
	"context"
	"log/slog"
	"time"
)

// Scheduler menjalankan RunDue secara berkala.
//
// Cuma satu goroutine dengan satu time.Ticker. Tidak ada worker pool, tidak ada
// errgroup: jumlah rule-nya sedikit dan pekerjaannya cepat.
type Scheduler struct {
	service  RecurringServicer
	interval time.Duration
	logger   *slog.Logger
}

func NewScheduler(service RecurringServicer, interval time.Duration, logger *slog.Logger) *Scheduler {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{service: service, interval: interval, logger: logger}
}

// Start menjalankan scheduler di background sampai ctx dibatalkan.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		s.runOnce(ctx) // jalan sekali saat startup, menyusul yang terlewat

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("scheduler berhenti")
				return
			case <-ticker.C:
				s.runOnce(ctx)
			}
		}
	}()
}

func (s *Scheduler) runOnce(ctx context.Context) {
	result, err := s.service.RunDue(ctx)
	if err != nil {
		s.logger.Error("scheduler gagal mengambil rule", "error", err)
		return
	}

	if result.RulesChecked == 0 {
		return
	}

	s.logger.Info("scheduler jalan",
		"rules_checked", result.RulesChecked,
		"created", result.Created,
		"skipped", result.Skipped,
		"rules_failed", result.RulesFailed)
}
