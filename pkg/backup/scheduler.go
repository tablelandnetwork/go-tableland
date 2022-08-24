package backup

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	logger "github.com/rs/zerolog/log"
)

var log = logger.With().Str("component", "backup").Logger()

// Scheduler executes backups at a regular interval.
type Scheduler struct {
	Frequency      int // in minutes
	NotificationCh chan bool

	backuper       *Backuper
	notify         bool
	tickerInterval time.Duration

	// control
	close     chan struct{}
	closeOnce sync.Once
}

// BackuperOptions options needed to instantiate a backuper.
type BackuperOptions struct {
	SourcePath, BackupDir string
	Opts                  []Option
}

// NewScheduler creates a new backup scheduler.
func NewScheduler(frequency int, opts BackuperOptions, notify bool) (*Scheduler, error) {
	if frequency < 1 || frequency >= 1440 {
		return nil, errors.New("interval should be in [1,1440)")
	}

	backuper, err := NewBackuper(opts.SourcePath, opts.BackupDir, opts.Opts...)
	if err != nil {
		return nil, fmt.Errorf("new backuper: %s", err)
	}

	return &Scheduler{
		Frequency:      frequency,
		NotificationCh: make(chan bool),

		notify:   notify,
		backuper: backuper,
		close:    make(chan struct{}),

		tickerInterval: time.Minute,
	}, nil
}

// Run starts the scheduler and listens for a shutdown call.
func (s *Scheduler) Run() {
	log.Info().Msg("starting backup scheduler")

	// wait until next interval to start
	now, interval := time.Now(), time.Duration(s.Frequency)*s.tickerInterval
	wait := now.Truncate(interval).Add(interval).Sub(now)

	for {
		select {
		case <-s.close:
			log.Info().Msg("closing backup scheduler")
			return
		case <-time.After(wait):
			startTime := time.Now()
			s.backup()
			if s.notify {
				s.NotificationCh <- true
			}
			wait = time.Duration(s.Frequency)*s.tickerInterval - time.Since(startTime)
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

func (s *Scheduler) backup() {
	if err := s.backuper.Init(); err != nil {
		log.Error().Err(err).Msg("initializing backuper")
		return
	}
	result, err := s.backuper.Backup(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("backup failed")
		return
	}
	log.Info().
		Str("path", result.Path).
		Int64("elapsed_time", result.ElapsedTime.Milliseconds()).
		Int64("elapsed_time_vacuum", result.VacuumElapsedTime.Milliseconds()).
		Int64("size", result.Size).
		Int64("size_vacuum", result.SizeAfterVacuum).
		Int64("size_compression", result.SizeAfterCompression).
		Msg("backup succeeded")

	if err := s.backuper.Close(); err != nil {
		log.Error().Err(err).Msg("closing backup")
		return
	}
}
