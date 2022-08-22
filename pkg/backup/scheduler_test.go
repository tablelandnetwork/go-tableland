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

	backuper, err := NewBackuper(controlDB.Path(), backupDir, []Option{WithVacuum(true)}...)
	require.NoError(t, err)

	scheduler := NewScheduler(2*time.Second, backuper, true)
	go scheduler.Run()

	var counter int
	for counter < 5 {
		<-scheduler.NotificationCh
		counter++
	}
	scheduler.Shutdown()
	requireFileCount(t, backupDir, counter)

	t.Cleanup(func() {
		require.NoError(t, controlDB.Close())
	})
}
