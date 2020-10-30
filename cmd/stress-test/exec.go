package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
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

	clients := newClients(getRPCList(ctx))
	if len(clients) == 0 {
		return errors.New("no rpc url set")
	}

	var (
		client        = clients[0]
		mainAccount   = newAccount(ctx.GlobalString(privKeyFlag.Name))
		accountAmount = ctx.Int(accountNumberFlag.Name)
		total         = ctx.Int(totalTxsFlag.Name)
		threads       = ctx.Int(threadsFlag.Name)
	)

	if total < accountAmount {
		return errors.New("total tx amount should bigger than account amount")
	}

	first := false
	var accounts []*bind.TransactOpts
	var toGen int
	keys, err := loadAccounts(getStorePath())
	if err != nil {
		log.Warn("load accounts failed", "err", err)
		first = true
		toGen = accountAmount
	}
	log.Info("load original accounts", "amount", len(keys))

	if !first && accountAmount > len(keys) {
		toGen = accountAmount - len(keys)
	}

	if len(keys) > 0 {
		accounts = append(accounts, newAccounts(keys)...)
	}

	if toGen > 0 {
		genKeys, genAccounts := generateRandomAccounts(toGen)
		log.Info("generate accounts over", "generated", len(genAccounts))

		accounts = append(accounts, genAccounts...)
		if first {
			if err := writeAccounts(getStorePath(), genKeys); err != nil {
				return err
			}
		} else {
			if err := appendAccounts(getStorePath(), genKeys); err != nil {
				return err
			}
		}

		// send this accounts hb and hsct.
		// send ether from main account to random account
		log.Info("send hb and token to test account")
		amount := big.NewInt(params.Ether)
		amount.Mul(amount, big.NewInt(100))

		// send hb for normal hb transfer test or pay gas fees
		sendEtherToRandomAccount(mainAccount, accounts, amount, common.Address{}, client)

		// send token to accounts.
		amount.Div(amount, divisor(defaultDecimal-decimal))
		sendEtherToRandomAccount(mainAccount, accounts, amount, token, client)
	}

	accounts = accounts[:accountAmount]

	// generate signed transactions
	amount := big.NewInt(params.Ether)
	amount.Div(amount, big.NewInt(1e+3))
	if (token != common.Address{}) {
		amount.Div(amount, divisor(defaultDecimal-decimal))
	}
	txs := generateSignedTransactions(total, accounts, amount, token, client)
	log.Info("generate txs over", "total", len(txs))

	currentBlock, _ := client.BlockByNumber(context.Background(), nil)
	log.Info("current block", "number", currentBlock.Number())

	// send txs
	start := time.Now()
	stressSendTransactions(txs, threads, clients, client)
	log.Info("send transaction over", "cost(milliseconds)", time.Now().Sub(start).Milliseconds())

	return nil
}
