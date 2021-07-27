package gasprice

import (
	"context"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	tenthGwei = big.NewInt(1e8)
	gwei      = big.NewInt(1e9)
)

type Prediction struct {
	cfg          *Config
	txCnts       *Stats // tx count statistics of few latest blocks
	backend      OracleBackend
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription
	pool         *core.TxPool

	predis        []uint // gas price prediction in gwei, currently will be 3 items, from hight(fast) to low(slow)
	lockPredis    sync.RWMutex
	wg            sync.WaitGroup
	blockGasLimit uint64
}

func NewPrediction(cfg Config, backend OracleBackend, pool *core.TxPool) *Prediction {
	if cfg.Blocks == 0 || cfg.FastFactor == 0 {
		//some test case offers no config
		return &Prediction{
			predis: make([]uint, 3),
		}
	}
	p := &Prediction{
		cfg:         &cfg,
		backend:     backend,
		chainHeadCh: make(chan core.ChainHeadEvent),
		pool:        pool,
	}
	price := wei2GWei(cfg.Default)
	p.predis = []uint{price * 2, price, price}

	// init txCnts
	p.initTxCnts()

	//subscripts chain head events
	p.chainHeadSub = backend.SubscribeChainHeadEvent(p.chainHeadCh)
	p.wg.Add(1)
	go p.loop()

	log.Info("Prediction started", "checkBlocks", cfg.Blocks, "Interval", cfg.PredictIntervalSecs,
		"ff", cfg.FastFactor, "mf", cfg.MedianFactor, "lf", cfg.LowFactor, "minMi", cfg.MinMedianIndex,
		"minLi", cfg.MinLowIndex, "fp", cfg.FastPercentile, "mp", cfg.MeidanPercentile, "minCnt", cfg.MinTxCntPerBlock)
	return p
}

// Stop stops the prediction loop
func (p *Prediction) Stop() {
	if p.chainHeadSub == nil {
		return
	}
	p.chainHeadSub.Unsubscribe()
	p.wg.Wait()
	log.Info("prediction quit")
}

// CurrentPrices returns the current prediction about gas price in gwei;
// the results should be readonly, and the reason didn't do a copy is that there's no necessary
func (p *Prediction) CurrentPrices() []uint {
	p.lockPredis.RLock()
	defer p.lockPredis.RUnlock()
	prices := p.predis
	return prices
}

func (p *Prediction) initTxCnts() {
	cnts := make([]int, p.cfg.Blocks)
	ctx := context.Background()
	head, _ := p.backend.HeaderByNumber(context.Background(), rpc.LatestBlockNumber)
	num := head.Number.Uint64()
	if num > uint64(p.cfg.Blocks) {
		for i, j := num-uint64(p.cfg.Blocks), 0; i < num; i, j = i+1, j+1 {
			block, err := p.backend.BlockByNumber(ctx, rpc.BlockNumber(i))
			if err != nil {
				log.Warn("Prediction, get block by number failed", "err", err)
				continue
			}
			cnts[j] = block.Transactions().Len()
		}
	} else if num > 0 {
		i := 1
		for ; i <= int(num); i++ {
			block, err := p.backend.BlockByNumber(ctx, rpc.BlockNumber(i))
			if err != nil {
				log.Warn("Prediction, get block by number failed", "err", err)
				continue
			}
			cnts[i-1] = block.Transactions().Len()
		}
		for ; i < p.cfg.Blocks; i++ {
			cnts[i] = cnts[i-1]
		}
	}
	p.txCnts = NewStats(cnts)

	//gas limit
	p.blockGasLimit = head.GasLimit
}

func (p *Prediction) loop() {
	// do an updates first
	p.update()

	tick := time.NewTicker(time.Duration(p.cfg.PredictIntervalSecs) * time.Second)
	defer tick.Stop()
	defer p.wg.Done()

	for {
		select {
		case <-tick.C:
			p.update()
		case ev := <-p.chainHeadCh:
			head := ev.Block
			txcnt := len(head.Transactions())
			p.txCnts.Add(txcnt)
			p.blockGasLimit = head.GasLimit()
		case <-p.chainHeadSub.Err():
			log.Warn("prediction loop quitting")
			return
		}
	}
}

func (p *Prediction) update() {
	txs, err := p.pool.Pending(true)
	if err != nil {
		log.Error("failed to get pending transactions", "err", err)
		return
	}
	byprice := make(TxByPrice, 0, len(txs))
	for _, ts := range txs {
		byprice = append(byprice, ts...)
	}
	byprice = p.filteroutInvalid(byprice)
	sort.Sort(byprice)

	minPrice := wei2GWei(p.pool.GasPrice())
	prices := make([]uint, 3)

	pendingCnt := len(byprice)
	if pendingCnt == 0 {
		// no pending tx, use minimum prices
		prices = []uint{minPrice, minPrice, minPrice}
		p.updatePredis(prices)
		return
	}

	avgTxCnt := p.txCnts.Avg()
	if avgTxCnt < p.cfg.MinTxCntPerBlock {
		avgTxCnt = p.cfg.MinTxCntPerBlock
	}

	// fast price
	fi := p.cfg.FastFactor * avgTxCnt
	if pendingCnt <= fi {
		fi = pendingCnt * p.cfg.FastPercentile / 100
	}
	prices[0] = wei2GWei(byprice[fi].GasPrice()) // fast price
	// if the fast price is 1 gwei, and there are lots of pending transactions,
	// then raise the fast price to 2 gwei.
	if prices[0] == 1 && pendingCnt > fi {
		prices[0] = 2
	}
	// median price
	mi := max(p.cfg.MedianFactor*avgTxCnt, p.cfg.MinMedianIndex)
	if pendingCnt <= mi {
		mi = pendingCnt * p.cfg.MeidanPercentile / 100
	}
	prices[1] = wei2GWei(byprice[mi].GasPrice())

	// low price, notice the differentce
	li := max(p.cfg.LowFactor*avgTxCnt, p.cfg.MinLowIndex)
	if pendingCnt <= li {
		prices[2] = minPrice
	} else {
		prices[2] = wei2GWei(byprice[li].GasPrice())
	}
	// make it more moderation
	if pendingCnt > mi &&
		prices[0] > prices[1]+1 &&
		prices[1] == prices[2] {
		prices[1]++
	}

	p.updatePredis(prices)
}

func (p *Prediction) filteroutInvalid(txs TxByPrice) TxByPrice {
	maxgas := (p.blockGasLimit / 10) * 6
	maxlive := time.Duration(p.cfg.MaxValidPendingSecs) * time.Second
	i, j := 0, len(txs)
	for i < j {
		tx := txs[i]
		if tx.Gas() > maxgas ||
			time.Since(tx.LocalSeenTime()) > maxlive ||
			tx.GasTipCapIntCmp(gwei) < 0 {
			j--
			txs[i], txs[j] = txs[j], txs[i]
			continue
		}
		//valid
		i++
	}
	return txs[:j]
}

func (p *Prediction) updatePredis(prices []uint) {
	p.lockPredis.Lock()
	for i := 0; i < 3; i++ {
		p.predis[i] = prices[i]
	}
	p.lockPredis.Unlock()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func wei2GWei(w *big.Int) uint {
	if nil == w {
		return 1
	}
	tgwei := new(big.Int).Div(w, tenthGwei).Uint64()
	if tgwei < 10 {
		return 1
	}
	return (uint(tgwei)*2 - 10) / 10 // rounding
}
