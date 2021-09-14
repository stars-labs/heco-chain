package core

import (
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var (
	jamIndexMeter = metrics.NewRegisteredGauge("txpool/jamindex", nil)
)

var oneGwei = big.NewInt(1e9)

var DefaultJamConfig = TxJamConfig{
	PeriodsSecs:         3,
	JamSecs:             15,
	UnderPricedFactor:   3,
	PendingFactor:       1,
	MaxValidPendingSecs: 300,
}

type TxJamConfig struct {
	PeriodsSecs       int // how many seconds to do a reflesh to the jam index
	JamSecs           int // how many seconds for a tx pending that will meams a tx-jam.
	UnderPricedFactor int
	PendingFactor     int

	MaxValidPendingSecs int //
}

func (c *TxJamConfig) sanity() TxJamConfig {
	cfg := *c
	if cfg.PeriodsSecs < 1 {
		log.Info("JamConfig sanity PeriodsSecs", "old", cfg.PeriodsSecs, "new", DefaultJamConfig.PeriodsSecs)
		cfg.PeriodsSecs = DefaultJamConfig.PeriodsSecs
	}
	if cfg.JamSecs < 3 {
		log.Info("JamConfig sanity JamSecs", "old", cfg.JamSecs, "new", DefaultJamConfig.JamSecs)
		cfg.JamSecs = DefaultJamConfig.JamSecs
	}
	if cfg.UnderPricedFactor < 1 {
		log.Info("JamConfig sanity UnderPricedFactor", "old", cfg.UnderPricedFactor, "new", DefaultJamConfig.UnderPricedFactor)
		cfg.UnderPricedFactor = DefaultJamConfig.UnderPricedFactor
	}
	if cfg.PendingFactor < 1 {
		log.Info("JamConfig sanity PendingFactor", "old", cfg.PendingFactor, "new", DefaultJamConfig.PendingFactor)
		cfg.PendingFactor = DefaultJamConfig.PendingFactor
	}
	if cfg.MaxValidPendingSecs <= cfg.JamSecs {
		log.Info("JamConfig sanity MaxValidPendingSecs", "old", cfg.MaxValidPendingSecs, "new", DefaultJamConfig.MaxValidPendingSecs)
		cfg.MaxValidPendingSecs = DefaultJamConfig.MaxValidPendingSecs
	}
	return cfg
}

// txJamIndexer try to give a quantitative index to reflects the tx-jam.
type txJamIndexer struct {
	cfg  TxJamConfig
	pool *TxPool
	head *types.Header

	undCounter      *underPricedCounter
	currentJamIndex int

	pendingLock sync.Mutex
	jamLock     sync.RWMutex

	quit        chan struct{}
	chainHeadCh chan *types.Header
}

func newTxJamIndexer(cfg TxJamConfig, pool *TxPool) *txJamIndexer {
	cfg = (&cfg).sanity()

	indexer := &txJamIndexer{
		cfg:         cfg,
		pool:        pool,
		undCounter:  newUnderPricedCounter(cfg.PeriodsSecs),
		quit:        make(chan struct{}),
		chainHeadCh: make(chan *types.Header, 1),
	}

	go indexer.updateLoop()

	return indexer
}

// Stop stops the loop goroutines of this TxJamIndexer
func (indexer *txJamIndexer) Stop() {
	indexer.undCounter.Stop()
	close(indexer.quit)
}

// JamIndex returns the current jam index
func (indexer *txJamIndexer) JamIndex() int {
	indexer.jamLock.RLock()
	defer indexer.jamLock.RUnlock()
	return indexer.currentJamIndex
}

func (indexer *txJamIndexer) updateLoop() {
	tick := time.NewTicker(time.Second * time.Duration(indexer.cfg.PeriodsSecs))
	defer tick.Stop()

	for {
		select {
		case h := <-indexer.chainHeadCh:
			indexer.head = h
		case <-tick.C:
			d := indexer.undCounter.Sum()
			pendings, _ := indexer.pool.Pending(true)
			if d == 0 && len(pendings) == 0 {
				break
			}
			// flatten
			var p int
			max := indexer.cfg.MaxValidPendingSecs
			jamsecs := indexer.cfg.JamSecs
			maxGas := uint64(10000000)
			if indexer.head != nil {
				maxGas = (indexer.head.GasLimit / 10) * 6
			}
			durs := make([]time.Duration, 0, 1024)
			for _, txs := range pendings {
				for _, tx := range txs {
					// filtering
					if tx.GasPrice().Cmp(oneGwei) < 0 ||
						tx.Gas() > maxGas {
						continue
					}

					dur := time.Since(tx.LocalSeenTime())
					sec := int(dur / time.Second)
					if sec > max {
						continue
					}

					durs = append(durs, dur)
					if sec >= jamsecs {
						p += sec / jamsecs
					}
				}
			}
			nTotal := len(durs)

			if nTotal == 0 {
				p = 0
			} else {
				p = 100 * p / nTotal
			}

			idx := d*indexer.cfg.UnderPricedFactor + p*indexer.cfg.PendingFactor
			indexer.jamLock.Lock()
			indexer.currentJamIndex = idx
			indexer.jamLock.Unlock()
			jamIndexMeter.Update(int64(idx))

			var dists []time.Duration
			sort.Slice(durs, func(i, j int) bool {
				return durs[i] < durs[j]
			})
			if nTotal > 10 {
				dists = append(dists, durs[0])
				for i := 1; i < 10; i++ {
					dists = append(dists, durs[nTotal*i/10])
				}
				dists = append(dists, durs[nTotal-1])
			} else {
				dists = durs
			}

			log.Trace("TxJamIndexer", "jamIndex", idx, "d", d, "p", p, "n", nTotal, "dists", dists)
		case <-indexer.quit:
			return
		}
	}
}

func (indexer *txJamIndexer) UpdateHeader(h *types.Header) {
	indexer.chainHeadCh <- h
}

func (indexer *txJamIndexer) UnderPricedInc() {
	indexer.undCounter.Inc()
}

type underPricedCounter struct {
	counts  []int // the lenght of this slice is 2 times of periodSecs
	periods int   //how many periods to cache, each period cache records of 0.5 seconds.
	idx     int   //current index
	sum     int   //current sum

	inCh       chan struct{}
	quit       chan struct{}
	queryCh    chan struct{}
	queryResCh chan int
}

func newUnderPricedCounter(periodSecs int) *underPricedCounter {
	c := &underPricedCounter{
		counts:     make([]int, 2*periodSecs),
		periods:    2 * periodSecs,
		inCh:       make(chan struct{}, 10),
		quit:       make(chan struct{}),
		queryCh:    make(chan struct{}),
		queryResCh: make(chan int),
	}
	go c.loop()
	return c
}

func (c *underPricedCounter) loop() {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			c.idx = (c.idx + 1) % c.periods
			c.sum -= c.counts[c.idx]
			c.counts[c.idx] = 0
		case <-c.inCh:
			c.counts[c.idx]++
			c.sum++
		case <-c.queryCh:
			c.queryResCh <- c.sum
		case <-c.quit:
			return
		}
	}
}

func (c *underPricedCounter) Sum() int {
	var sum int
	c.queryCh <- struct{}{}
	sum = <-c.queryResCh
	return sum
}

func (c *underPricedCounter) Inc() {
	c.inCh <- struct{}{}
}

func (c *underPricedCounter) Stop() {
	close(c.quit)
}
