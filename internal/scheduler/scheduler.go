package scheduler

import (
	"log"
	"sync"
	"time"

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
	// 延迟执行首轮采集，避免与用户首次打开页面的请求争抢资源，首屏加载更快
	const startupDelay = 15 * time.Second
	time.AfterFunc(startupDelay, func() {
		go s.runOnce()
	})
}

// RunOnce 对外暴露的单次执行入口，方便手动触发采集
func (s *Scheduler) RunOnce() {
	s.runOnce()
}

func (s *Scheduler) runOnce() {
	log.Println("start collect job...")

	var wg sync.WaitGroup
	for _, f := range s.fetchers {
		fetcher := f
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fetcher.Name()
			log.Printf("fetch from %s...", name)
			items, err := fetcher.Fetch()
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
			// 条数 = 本轮采集解析到的数量（非“新增数”，已存在会更新）
			log.Printf("%s done, fetched=%d saved=%d items", name, len(items), len(processed))
		}()
	}

	wg.Wait()
	log.Println("collect job done (all sources)")
}

