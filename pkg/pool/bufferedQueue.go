package pool


type BufferedJobQueue chan Job

// returns true wether a new item will block or not
func (bq *BufferedJobQueue) Blocked() bool {
	return len(*bq) == cap(*bq)
}