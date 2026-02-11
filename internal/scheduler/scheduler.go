package scheduler

import (
	"log"

	"github.com/LJTian/TrendingHub/internal/collector"
	"github.com/LJTian/TrendingHub/internal/processor"
	"github.com/LJTian/TrendingHub/internal/storage"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron      *cron.Cron
	fetchers  []collector.Fetcher
	processor *processor.SimpleProcessor
	store     *storage.Store
}

func New(spec string, fetchers []collector.Fetcher, p *processor.SimpleProcessor, store *storage.Store) (*Scheduler, error) {
	c := cron.New()

	s := &Scheduler{
		cron:      c,
		fetchers:  fetchers,
		processor: p,
		store:     store,
	}

	_, err := c.AddFunc(spec, s.runOnce)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
	// 启动时先异步执行一轮，避免等待下一次 cron 周期
	go s.runOnce()
}

// RunOnce 对外暴露的单次执行入口，方便手动触发采集
func (s *Scheduler) RunOnce() {
	s.runOnce()
}

func (s *Scheduler) runOnce() {
	log.Println("start collect job...")

	var all []collector.NewsItem

	for _, f := range s.fetchers {
		log.Printf("fetch from %s...", f.Name())
		items, err := f.Fetch()
		if err != nil {
			log.Printf("fetch %s error: %v", f.Name(), err)
			continue
		}
		all = append(all, items...)
	}

	processed := s.processor.Process(all)
	if len(processed) == 0 {
		log.Println("no news to save")
		return
	}

	if err := s.store.SaveBatch(processed); err != nil {
		log.Printf("save batch error: %v", err)
		return
	}

	log.Printf("collect job done, saved %d items", len(processed))
}

