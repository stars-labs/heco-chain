package main

import (
	"context"
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

// newRandomAccount generates a random ethereum account with bind transactor
func newRandomAccount() *bind.TransactOpts {
	key, err := crypto.GenerateKey()
	if err != nil {
		utils.Fatalf("Failed to genreate random key: %v", err)
	}

	return bind.NewKeyedTransactor(key)
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
		signedTx, _ := mainAccount.Signer(types.HomesteadSigner{}, mainAccount.From, generateTx(nonce, account.From, amount, token))
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

	for _, account := range accounts {
		currentNonce, err := client.NonceAt(context.Background(), account.From, nil)
		if err != nil {
			utils.Fatalf("Failed to get account nonce: %v", err)
		}

		for i := 0; i < total/len(accounts); i++ {
			signedTx, _ := account.Signer(types.HomesteadSigner{}, account.From, generateTx(currentNonce, receiver, amount, token))
			txs = append(txs, signedTx)

			currentNonce++
		}
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
