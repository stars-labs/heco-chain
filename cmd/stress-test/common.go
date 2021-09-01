package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"gopkg.in/urfave/cli.v1"
)

const (
	separator = ","
)

type buildTxFn func(nonce uint64, to common.Address, amount *big.Int, token common.Address) *types.Transaction

// newClient creates a client with specified remote URL.
func newClient(url string) *ethclient.Client {
	client, err := ethclient.Dial(url)
	if err != nil {
		utils.Fatalf("Failed to connect to Ethereum node: %v", err)
	}

	return client
}

func newClients(urls []string) []*ethclient.Client {
	clients := make([]*ethclient.Client, 0)

	for _, url := range urls {
		client, err := ethclient.Dial(url)
		if err != nil {
			continue
		}

		clients = append(clients, client)
	}

	return clients
}

func getRPCList(ctx *cli.Context) []string {
	urlStr := ctx.GlobalString(nodeURLFlag.Name)
	list := make([]string, 0)

	for _, url := range strings.Split(urlStr, separator) {
		if url = strings.Trim(url, " "); len(url) != 0 {
			list = append(list, url)
		}
	}
	if len(list) == 0 {
		utils.Fatalf("Failed to find any valid rpc url in: %v", urlStr)
	}

	return list
}

// newAccount creates a ethereum account with bind transactor by plaintext key string in hex format .
func newAccount(hexKey string) *bind.TransactOpts {
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		utils.Fatalf("Failed to get privkey by hex key: %v", err)
	}

	return bind.NewKeyedTransactor(key)
}

func newAccounts(keys []*ecdsa.PrivateKey) []*bind.TransactOpts {
	accounts := make([]*bind.TransactOpts, 0)

	for _, k := range keys {
		accounts = append(accounts, bind.NewKeyedTransactor(k))
	}

	return accounts
}

// newRandomAccount generates a random ethereum account with bind transactor
func newRandomAccount() *bind.TransactOpts {
	key, err := crypto.GenerateKey()
	if err != nil {
		utils.Fatalf("Failed to genreate random key: %v", err)
	}

	return bind.NewKeyedTransactor(key)
}

// generateRandomAccounts generates servial random accounts
// concurrent do this if account amount is to big.
func generateRandomAccounts(amount int) ([]*ecdsa.PrivateKey, []*bind.TransactOpts) {
	keys := make([]*ecdsa.PrivateKey, 0)
	result := make([]*bind.TransactOpts, 0)

	workFn := func(start, end int, data ...interface{}) []interface{} {
		tmpAccounts := make([]interface{}, 0)
		for i := start; i < end; i++ {
			key, _ := crypto.GenerateKey()

			tmpAccounts = append(tmpAccounts, key)
		}

		return tmpAccounts
	}
	for _, account := range concurrentWork(amount/jobsPerThread+1, amount, workFn, nil) {
		keys = append(keys, account.(*ecdsa.PrivateKey))
		result = append(result, bind.NewKeyedTransactor(account.(*ecdsa.PrivateKey)))
	}

	return keys, result
}

// newSendEtherTransaction creates a normal transfer transaction.
func newHBStansferTransaction(nonce uint64, to common.Address, amount *big.Int) *types.Transaction {
	gasPrice := big.NewInt(10)
	gasPrice.Mul(gasPrice, big.NewInt(params.GWei))

	return types.NewTransaction(nonce, to, amount, hbTransferLimit, gasPrice, []byte{})
}

func newTokenTransferTransaction(nonce uint64, token, to common.Address, amount *big.Int) *types.Transaction {
	gasPrice := big.NewInt(10)
	gasPrice.Mul(gasPrice, big.NewInt(params.GWei))

	return types.NewTransaction(nonce, token, new(big.Int), tokenTransferLimit, gasPrice, packData(to, amount))
}

func generateTx(nonce uint64, to common.Address, amount *big.Int, token common.Address) *types.Transaction {
	if (token == common.Address{}) {
		return newHBStansferTransaction(nonce, to, amount)
	}

	return newTokenTransferTransaction(nonce, token, to, amount)
}

func packData(to common.Address, amount *big.Int) []byte {
	data := make([]byte, 68)

	sig, _ := hex.DecodeString(tokenTransferSig)
	copy(data[:4], sig[:])

	toBytes := to.Bytes()
	copy(data[36-len(toBytes):36], toBytes[:])

	vBytes := amount.Bytes()
	copy(data[68-len(vBytes):], vBytes[:])

	return data
}

func sendEtherToRandomAccount(mainAccount *bind.TransactOpts, accounts []*bind.TransactOpts, amount *big.Int, token common.Address, client *ethclient.Client) {
	nonce, err := client.NonceAt(context.Background(), mainAccount.From, nil)
	if err != nil {
		utils.Fatalf("Failed to get account nonce: %v", err)
	}

	var lastHash common.Hash
	for _, account := range accounts {
		signedTx, _ := mainAccount.Signer(mainAccount.From, generateTx(nonce, account.From, amount, token))
		if err := client.SendTransaction(context.Background(), signedTx); err != nil {
			utils.Fatalf("Failed to send ether to random account: %v", err)
		}

		lastHash = signedTx.Hash()
		nonce++
	}

	waitForTx(lastHash, client)
}

// generateSignedTransactions generates transactions.
func generateSignedTransactions(total int, accounts []*bind.TransactOpts, amount *big.Int, token common.Address, client *ethclient.Client) (txs []*types.Transaction) {
	// total txs
	workFn := func(start, end int, data ...interface{}) []interface{} {
		// like 15 threads, 15 account, 1000 txs
		account := accounts[start/(total/len(accounts))]
		currentNonce, err := client.NonceAt(context.Background(), account.From, nil)
		if err != nil {
			utils.Fatalf("Failed to get account nonce: %v", err)
		}

		result := make([]interface{}, 0)
		for i := start; i < end; i++ {
			signedTx, _ := account.Signer(account.From, generateTx(currentNonce, receiver, amount, token))
			result = append(result, signedTx)

			currentNonce++
		}

		return result
	}

	// accounts
	result := concurrentWork(len(accounts), total, workFn, nil)
	for _, tx := range result {
		txs = append(txs, tx.(*types.Transaction))
	}

	return
}

func waitForTx(hash common.Hash, client *ethclient.Client) {
	log.Info("wait for transaction packed", "tx", hash.Hex())
	for {
		receipt, _ := client.TransactionReceipt(context.Background(), hash)
		if receipt != nil {
			log.Info("transaction packed!")
			return
		}

		time.Sleep(time.Second)
	}
}

func stressSendTransactions(txs []*types.Transaction, threads int, clients []*ethclient.Client, client *ethclient.Client) {
	jobsPerThreadTmp := len(txs) / threads

	workFn := func(start, end int, data ...interface{}) []interface{} {
		c := clients[(start/jobsPerThreadTmp)%len(clients)]

		for i := start; i < end; i++ {
			if err := c.SendTransaction(context.Background(), txs[i]); err != nil {
				log.Error("send tx failed", "err", err)
			}
		}

		return []interface{}{}
	}

	concurrentWork(threads, len(txs), workFn, nil)
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

type workFunc func(start, end int, data ...interface{}) []interface{}

func concurrentWork(threads, totalWorks int, job workFunc, data ...interface{}) []interface{} {

	dataChan := make(chan []interface{})
	doJobFunc := func(i int) {
		start := i * totalWorks / threads
		// cal end of the work
		end := (i + 1) * totalWorks / threads
		if end > totalWorks {
			end = totalWorks
		}

		dataChan <- job(start, end, data)
	}

	for i := 0; i < threads; i++ {
		go doJobFunc(i)
	}

	// wait for all job done
	doneJob := 0
	result := make([]interface{}, 0)
	for {
		if doneJob == threads {
			break
		}

		select {
		case data := <-dataChan:
			result = append(result, data...)
			doneJob++
		}
	}

	return result
}
