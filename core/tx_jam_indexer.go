package core

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var DefaultJamConfig = TxJamConfig{
	PeriodsSecs:         3,
	JamSecs:             10,
	UnderPricedFactor:   5,
	PendingFactor:       10,
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

// TxJamIndexer try to give a quantitative index to reflects the tx-jam.
type TxJamIndexer struct {
	cfg TxJamConfig

	undCounter      *underPricedCounter
	pendingTxs      map[common.Hash]time.Time //the time a tx was promoted to the pending list
	currentJamIndex int

	pendingLock sync.Mutex
	jamLock     sync.RWMutex

	quit  chan struct{}
	inCh  chan types.Transactions
	outCh chan types.Transactions
}

func NewTxJamIndexer(cfg TxJamConfig) *TxJamIndexer {
	cfg = (&cfg).sanity()
	//todo

	indexer := &TxJamIndexer{
		cfg:        cfg,
		undCounter: newUnderPricedCounter(cfg.PeriodsSecs),
		pendingTxs: make(map[common.Hash]time.Time),
		quit:       make(chan struct{}),
		inCh:       make(chan types.Transactions, 10),
		outCh:      make(chan types.Transactions, 10),
	}

	go indexer.updateLoop()
	go indexer.mainLoop()

	return indexer
}

// Stop stops the loop goroutines of this TxJamIndexer
func (indexer *TxJamIndexer) Stop() {
	indexer.undCounter.Stop()
	close(indexer.quit)
}

// JamIndex returns the current jam index
func (indexer *TxJamIndexer) JamIndex() int {
	indexer.jamLock.RLock()
	defer indexer.jamLock.RUnlock()
	return indexer.currentJamIndex
}

func (indexer *TxJamIndexer) updateLoop() {
	tick := time.NewTicker(time.Second * time.Duration(indexer.cfg.PeriodsSecs))
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			d := indexer.undCounter.Sum()
			indexer.pendingLock.Lock()
			nTotal := len(indexer.pendingTxs)
			var p int
			now := time.Now()
			max := indexer.cfg.MaxValidPendingSecs
			jamsecs := indexer.cfg.JamSecs
			for _, start := range indexer.pendingTxs {
				sec := int(now.Sub(start) / time.Second)
				if sec <= max && sec >= jamsecs {
					p += sec / jamsecs
				}
			}
			indexer.pendingLock.Unlock()

			log.Trace("TxJamIndexer update lock", "elapse", time.Since(now))
			if nTotal == 0 {
				p = 0
			} else {
				p = 100 * p / nTotal
			}

			idx := d*indexer.cfg.UnderPricedFactor + p*indexer.cfg.PendingFactor
			indexer.jamLock.Lock()
			indexer.currentJamIndex = idx
			indexer.jamLock.Unlock()
			log.Trace("TxJamIndexer", "jamIndex", idx)
		case <-indexer.quit:
			return
		}
	}
}

func (indexer *TxJamIndexer) mainLoop() {
	for {
		select {
		case <-indexer.quit:
			return
		case txs := <-indexer.inCh:
			if len(txs) > 0 {
				indexer.pendingLock.Lock()
				for _, tx := range txs {
					indexer.pendingTxs[tx.Hash()] = time.Now()
				}
				indexer.pendingLock.Unlock()
			}
		case txs := <-indexer.outCh:
			if len(txs) > 0 {
				indexer.pendingLock.Lock()
				for _, tx := range txs {
					delete(indexer.pendingTxs, tx.Hash())
				}
				indexer.pendingLock.Unlock()
			}
		}
	}
}

func (indexer *TxJamIndexer) PendingIn(txs types.Transactions) {
	indexer.inCh <- txs
}

func (indexer *TxJamIndexer) PendingOut(txs types.Transactions) {
	indexer.outCh <- txs
}

func (indexer *TxJamIndexer) UnderPricedInc() {
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
