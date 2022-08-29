package backup

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	logger "github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

var log = logger.With().Str("component", "backup").Logger()

// Scheduler executes backups at a regular interval.
type Scheduler struct {
	NotificationCh chan error

	notify          bool
	backuper        *Backuper
	tickerFrequency time.Duration

	// control
	close     chan struct{}
	closeOnce sync.Once

	// metrics
	mLastExecution time.Time
}

// BackuperOptions options needed to instantiate a backuper.
type BackuperOptions struct {
	SourcePath, BackupDir string
	Opts                  []Option
}

// NewScheduler creates a new backup scheduler.
func NewScheduler(frequency int, opts BackuperOptions, notify bool) (*Scheduler, error) {
	if frequency < 1 || frequency >= 1440 {
		return nil, errors.New("frequency should be in [1,1440)")
	}

	backuper, err := NewBackuper(opts.SourcePath, opts.BackupDir, opts.Opts...)
	if err != nil {
		return nil, fmt.Errorf("new backuper: %s", err)
	}

	s := &Scheduler{
		NotificationCh:  make(chan error),
		notify:          notify,
		backuper:        backuper,
		tickerFrequency: time.Duration(frequency) * time.Minute,
		close:           make(chan struct{}),
	}
	if err := s.initMetrics(); err != nil {
		return nil, errors.Errorf("init metrics: %s", err)
	}
	return s, nil
}

// Run starts the scheduler and listens for a shutdown call.
func (s *Scheduler) Run() {
	log.Info().Msg("starting backup scheduler")

	// wait until next interval to start
	now, interval := time.Now(), s.tickerFrequency
	wait := now.Truncate(interval).Add(interval).Sub(now)

	for {
		select {
		case <-s.close:
			log.Info().Msg("closing backup scheduler")
			return
		case <-time.After(wait):
			startTime := time.Now()
			err := s.backup()
			if s.notify {
				s.NotificationCh <- err
			}
			// It executes again next tick independent of error
			// We could implement a retry strategy if needed
			wait = s.tickerFrequency - time.Since(startTime)
		}
	}
}

// Shutdown gracefully shutdowns the scheduler.
func (s *Scheduler) Shutdown() {
	s.closeOnce.Do(func() {
		s.close <- struct{}{}
		close(s.close)
	})
}

func (s *Scheduler) backup() error {
	result, err := s.backuper.Backup(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("backup failed")
		return errors.Errorf("backup failed: %s", err)
	}

	if err := s.backuper.Close(); err != nil {
		log.Error().Err(err).Msg("closing backup")
		return errors.Errorf("closing backup: %s", err)
	}

	log.Info().
		Str("path", result.Path).
		Str("timestamp", result.Timestamp.Format(time.RFC3339)).
		Int64("elapsed_time", result.ElapsedTime.Milliseconds()).
		Int64("elapsed_time_vacuum", result.VacuumElapsedTime.Milliseconds()).
		Int64("elapsed_time_compression", result.CompressionElapsedTime.Milliseconds()).
		Int64("size", result.Size).
		Int64("size_vacuum", result.SizeAfterVacuum).
		Int64("size_compression", result.SizeAfterCompression).
		Msg("backup succeeded")

	s.mLastExecution = result.Timestamp

	return nil
}

func (s *Scheduler) initMetrics() error {
	meter := global.MeterProvider().Meter("tableland")
	mLastExecution, err := meter.AsyncInt64().Gauge("tableland.backup.last_execution")
	if err != nil {
		return fmt.Errorf("registering last execution gauge: %s", err)
	}

	if err := meter.RegisterCallback([]instrument.Asynchronous{mLastExecution}, func(ctx context.Context) {
		if !s.mLastExecution.IsZero() {
			mLastExecution.Observe(ctx, s.mLastExecution.Unix())
		}
	}); err != nil {
		return fmt.Errorf("registering callback on instruments: %s", err)
	}

	return nil
}
