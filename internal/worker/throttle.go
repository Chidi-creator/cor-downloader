package worker

import "time"

// progressUpdateInterval is the minimum time between progress writes to
// Postgres - io.Copy calls onProgress on every chunk (potentially hundreds
// of times per second), but a database row doesn't need updating that
// often.
const progressUpdateInterval = 250 * time.Millisecond

// shouldSendProgressUpdate decides whether enough time has passed since the
// last update to be worth writing again, or whether this is the final
// chunk - which always gets written, so the stored progress doesn't get
// stuck below the true total.
func shouldSendProgressUpdate(now, last time.Time, downloaded, total int64) bool {
	if total > 0 && downloaded >= total {
		return true
	}
	return now.Sub(last) >= progressUpdateInterval
}
