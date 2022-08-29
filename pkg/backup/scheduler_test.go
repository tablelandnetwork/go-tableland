package backup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	t.Parallel()
	backupDir := backupDir(t)
	controlDB := createControlDatabase(t)

	interval := 2 // for test, every 2 seconds it will generate a backup file
	scheduler, err := NewScheduler(interval, BackuperOptions{
		SourcePath: controlDB.Path(),
		BackupDir:  backupDir,
		Opts:       []Option{WithVacuum(true)},
	}, true)
	require.NoError(t, err)

	scheduler.tickerFrequency = time.Duration(interval) * time.Second // for test, ticks every second
	go scheduler.Run()

	var counter int
	for range scheduler.NotificationCh {
		counter++
		if counter == 5 {
			break
		}
	}
	scheduler.Shutdown()
	requireFileCount(t, backupDir, counter)

	t.Cleanup(func() {
		require.NoError(t, controlDB.Close())
	})
}
