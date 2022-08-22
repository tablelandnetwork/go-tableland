package backup

import (
	"context"
	"sync"
	"time"

	logger "github.com/rs/zerolog/log"
)

var log = logger.With().Str("component", "backup").Logger()

// Scheduler executes backups at a regular interval.
type Scheduler struct {
	Interval       time.Duration
	NotificationCh chan bool

	backuper *Backuper
	notify   bool
	// control
	close     chan struct{}
	closeOnce sync.Once
}

// NewScheduler creates a new backup scheduler.
func NewScheduler(interval time.Duration, backuper *Backuper, notify bool) *Scheduler {
	return &Scheduler{
		Interval:       interval,
		NotificationCh: make(chan bool),

		notify:   notify,
		backuper: backuper,
		close:    make(chan struct{}),
	}
}

// Run starts the scheduler and listens for a shutdown call.
func (s *Scheduler) Run() {
	log.Info().Msg("starting backup scheduler")

	period := s.Interval
	for {
		select {
		case <-s.close:
			log.Info().Msg("closing backup scheduler")
			return
		case <-time.After(period):
		}

		startTime := time.Now()
		s.backup()
		if s.notify {
			s.NotificationCh <- true
		}
		period = s.Interval - time.Since(startTime)
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
	}
	result, err := s.backuper.Backup(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("backup failed")
	}
	log.Info().
		Str("path", result.Path).
		Int64("elapsed_time", result.ElapsedTime.Milliseconds()).
		Int64("elapsed_time_vacuum", result.VacuumElapsedTime.Milliseconds()).
		Int64("size", result.Size).
		Int64("size_vacuum", result.SizeAfterVacuum).
		Msg("backup succeeded")

	if err := s.backuper.Close(); err != nil {
		log.Error().Err(err).Msg("closing backup")
	}
}
