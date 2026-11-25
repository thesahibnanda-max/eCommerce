package paymentGateway

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"maragu.dev/goqite"
	"maragu.dev/goqite/jobs"
	_ "modernc.org/sqlite"
)

//go:embed queue_schema.sql
var queueSchemaSQL string

const (
	queueName          = "payments"
	queueMaxReceive    = 5
	busyTimeoutMillis  = 5000
	minRunnerPoll      = 10 * time.Millisecond
	maxRunnerPoll      = time.Second
	enqueueRetryWindow = 2 * time.Second
)

type jobHandler func(ctx context.Context, body []byte) error

type queueConfig struct {
	path         []string
	permission   os.FileMode
	leaseTimeout time.Duration
	pollInterval time.Duration
	concurrency  int
	logger       *slog.Logger
}

type jobQueue struct {
	db        *sql.DB
	queue     *goqite.Queue
	runner    *jobs.Runner
	cancel    context.CancelFunc
	done      chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
	closeErr  error
}

func newJobQueue(cfg queueConfig) (*jobQueue, error) {
	file := filepath.Join(cfg.path...)
	handle, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, cfg.permission)
	if err != nil {
		return nil, err
	}
	if err := handle.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", queueDSN(file))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(queueSchemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	queue := goqite.New(goqite.NewOpts{DB: db, Name: queueName, MaxReceive: queueMaxReceive, Timeout: cfg.leaseTimeout})
	runnerOpts := jobs.NewRunnerOpts{
		Extend:       cfg.leaseTimeout,
		Limit:        cfg.concurrency,
		PollInterval: clampDuration(cfg.pollInterval/2, minRunnerPoll, maxRunnerPoll),
		Queue:        queue,
	}
	if cfg.logger != nil {
		runnerOpts.Log = cfg.logger
	}
	return &jobQueue{db: db, queue: queue, runner: jobs.NewRunner(runnerOpts), done: make(chan struct{})}, nil
}

func (q *jobQueue) register(name string, handler jobHandler) {
	q.runner.Register(name, jobs.Func(handler))
}

func (q *jobQueue) start() {
	q.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		q.cancel = cancel
		go func() {
			defer close(q.done)
			q.runner.Start(ctx)
		}()
	})
}

func (q *jobQueue) enqueue(ctx context.Context, name string, message any, delay time.Duration) error {
	body, err := utils.FromObjectToStringJSON(message)
	if err != nil {
		return err
	}
	if delay < 0 {
		delay = 0
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), enqueueRetryWindow)
	defer cancel()
	if _, err := jobs.Create(sendCtx, q.queue, name, goqite.Message{Body: []byte(body), Delay: delay}); err != nil {
		return fmt.Errorf("paymentGateway: enqueue %s: %w", name, err)
	}
	return nil
}

func (q *jobQueue) close() error {
	q.closeOnce.Do(func() {
		if q.cancel != nil {
			q.cancel()
			<-q.done
		}
		q.closeErr = q.db.Close()
	})
	return q.closeErr
}

func queueDSN(path string) string {
	params := url.Values{}
	params.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMillis))
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_txlock", "immediate")
	return "file:" + path + "?" + params.Encode()
}

func clampDuration(value, low, high time.Duration) time.Duration {
	return max(low, min(value, high))
}
