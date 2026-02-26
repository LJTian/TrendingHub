package scheduler

import (
	"log"
	"sync"

	"github.com/LJTian/TrendingHub/internal/collector"
	"github.com/LJTian/TrendingHub/internal/processor"
	"github.com/LJTian/TrendingHub/internal/storage"
	"github.com/robfig/cron/v3"
)

// FetcherJob 将采集器与独立的 cron 调度绑定
type FetcherJob struct {
	Fetcher  collector.Fetcher
	CronSpec string
}

type Scheduler struct {
	cron      *cron.Cron
	jobs      []FetcherJob
	processor *processor.SimpleProcessor
	store     *storage.Store
}

func New(jobs []FetcherJob, p *processor.SimpleProcessor, store *storage.Store) (*Scheduler, error) {
	c := cron.New()

	s := &Scheduler{
		cron:      c,
		jobs:      jobs,
		processor: p,
		store:     store,
	}

	for _, job := range jobs {
		j := job
		if _, err := c.AddFunc(j.CronSpec, func() { s.runFetcher(j.Fetcher) }); err != nil {
			return nil, err
		}
		log.Printf("scheduled %s with cron: %s", j.Fetcher.Name(), j.CronSpec)
	}

	return s, nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
	go s.RunOnce()
}

// Cron 暴露底层 cron 实例，方便外部注册额外任务
func (s *Scheduler) Cron() *cron.Cron {
	return s.cron
}

// RunOnce 并发执行所有采集器一次
func (s *Scheduler) RunOnce() {
	log.Println("start collect job (all sources)...")
	var wg sync.WaitGroup
	for _, job := range s.jobs {
		j := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runFetcher(j.Fetcher)
		}()
	}
	wg.Wait()
	log.Println("collect job done (all sources)")
}

func (s *Scheduler) runFetcher(f collector.Fetcher) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("fetch %s panic recovered: %v", f.Name(), r)
		}
	}()
	name := f.Name()
	log.Printf("fetch from %s...", name)

	items, err := f.Fetch()
	if err != nil {
		log.Printf("fetch %s error: %v", name, err)
		return
	}
	if len(items) == 0 {
		log.Printf("fetch %s got 0 items", name)
		return
	}

	processed := s.processor.Process(items)
	if len(processed) == 0 {
		return
	}
	if err := s.store.SaveBatch(processed); err != nil {
		log.Printf("save %s batch error: %v", name, err)
		return
	}
	log.Printf("%s done, fetched=%d saved=%d items", name, len(items), len(processed))
}
