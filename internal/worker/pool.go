package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"cor-downloader/internal/queue"
)

// Pool runs a fixed number of goroutines that continuously pull jobs from
// the queue and process them, until ctx is cancelled.
type Pool struct {
	Queue       *queue.Queue
	Processor   *Processor
	Concurrency int
}

// Run blocks until ctx is cancelled, then waits for in-flight jobs to
// finish before returning.
func (p *Pool) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < p.Concurrency; i++ {
		wg.Add(1)
		workerNum := i
		go func() {
			defer wg.Done()
			p.runWorker(ctx, workerNum)
		}()
	}
	wg.Wait()
}

func (p *Pool) runWorker(ctx context.Context, workerNum int) {
	for {
		if ctx.Err() != nil {
			return
		}

		jobID, err := p.Queue.Pop(ctx, 5*time.Second)
		if errors.Is(err, queue.ErrEmpty) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("worker %d: pop failed: %v", workerNum, err)
			continue
		}

		log.Printf("worker %d: processing job %s", workerNum, jobID)
		if err := p.Processor.ProcessJob(ctx, jobID); err != nil {
			log.Printf("worker %d: job %s failed: %v", workerNum, jobID, err)
			continue
		}
		log.Printf("worker %d: job %s done", workerNum, jobID)
	}
}
