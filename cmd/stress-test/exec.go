package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"gopkg.in/urfave/cli.v1"
)

var commandStressTestNormal = cli.Command{
	Name:  "testNormal",
	Usage: "Send normal transfer transactions for stress test",
	Flags: []cli.Flag{
		nodeURLFlag,
		privKeyFlag,
		accountNumberFlag,
		totalTxsFlag,
		threadsFlag,
	},
	Action: utils.MigrateFlags(stressTestNormal),
}

var commandStressTestToken = cli.Command{
	Name:  "testToken",
	Usage: "Send token transfer transactions for stress test",
	Flags: []cli.Flag{
		nodeURLFlag,
		privKeyFlag,
		accountNumberFlag,
		totalTxsFlag,
		threadsFlag,
		tokenFlag,
		decimalFlag,
	},
	Action: utils.MigrateFlags(stressTestToken),
}

func stressTestNormal(ctx *cli.Context) error {
	return stressTest(ctx, common.Address{}, 0)
}

func stressTestToken(ctx *cli.Context) error {
	token := common.HexToAddress(ctx.String(tokenFlag.Name))
	decimal := ctx.Int(decimalFlag.Name)
	if decimal > 18 || decimal <= 0 {
		return fmt.Errorf("Unsupported decimal %d", decimal)
	}

	return stressTest(ctx, token, decimal)
}

func stressTest(ctx *cli.Context, token common.Address, decimal int) error {
	var (
		rpcList = getRPCList(ctx)
	)
	if len(rpcList) == 0 {
		return errors.New("no rpc url set")
	}

	var (
		client        = newClient(rpcList[0])
		mainAccount   = newAccount(ctx.GlobalString(privKeyFlag.Name))
		accountAmount = ctx.Int(accountNumberFlag.Name)
		total         = ctx.Int(totalTxsFlag.Name)
		threads       = ctx.Int(threadsFlag.Name)
	)

	if total < accountAmount {
		return errors.New("total tx amount should bigger than account amount")
	}

	// generate accounts
	accounts := make([]*bind.TransactOpts, 0)
	for i := 0; i < accountAmount; i++ {
		accounts = append(accounts, newRandomAccount())
	}

	// send ether from main account to random account
	amount := big.NewInt(params.Ether)
	amount.Mul(amount, big.NewInt(int64(total/len(accounts))))
	amount.Add(amount, fee)
	// send fee hb for accout to send transaction
	if (token != common.Address{}) {
		// 110 000 000
		amount.Div(amount, divisor(defaultDecimal-decimal))

		sendEtherToRandomAccount(mainAccount, accounts, amount, token, client)
		amount = new(big.Int).Set(fee)
	}
	// send hb for normal hb transfer test or pay gas fees
	sendEtherToRandomAccount(mainAccount, accounts, amount, common.Address{}, client)

	// generate signed transactions
	amount = big.NewInt(params.Ether)
	if (token != common.Address{}) {
		amount.Div(amount, divisor(defaultDecimal-decimal))
	}
	txs := generateSignedTransactions(total, accounts, amount, token, client)

	// send txs
	stressSendTransactions(txs, threads, rpcList, client)

	return nil
}

func stressSendTransactions(txs []*types.Transaction, threads int, rpcList []string, client *ethclient.Client) {
	wg := sync.WaitGroup{}
	wg.Add(threads)

	start := time.Now()
	for i := 0; i < threads; i++ {
		c := newClient(rpcList[i%len(rpcList)])

		go func(i int, c *ethclient.Client) {
			end := (i + 1) * len(txs) / threads
			if end > len(txs) {
				end = len(txs)
			}

			// send txs
			for _, tx := range txs[i*len(txs)/threads : end] {
				if err := c.SendTransaction(context.Background(), tx); err != nil {
					log.Error("send tx failed", "err", err)
				}
			}

			wg.Done()
		}(i, c)
	}
	wg.Wait()

	cost := time.Now().Sub(start).Milliseconds()
	log.Info("Stress test over", "try send tx amount", len(txs), "time used(milliseconds)", cost)
}

func divisor(decimal int) *big.Int {
	if decimal <= 0 {
		return big.NewInt(1)
	}

	d := big.NewInt(10)
	for i := 0; i < decimal; i++ {
		d.Mul(d, big.NewInt(10))
	}

	return d
}
