package main

import (
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/fdlimit"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"gopkg.in/urfave/cli.v1"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""
)

// const test params
var (
	receiver = common.HexToAddress("0x4Bee7F41037532509368b7B4CA8255b44Dd8Fb77")
	fee      = new(big.Int).Mul(big.NewInt(10), big.NewInt(params.Ether))

	hbTransferLimit    = uint64(21000)
	tokenTransferLimit = uint64(100000)
	tokenTransferSig   = "a9059cbb"

	defaultDecimal = 18

	jobsPerThread = 20

	storePath = ".keys"
)

var app *cli.App

func init() {
	app = flags.NewApp(gitCommit, gitDate, "ethereum checkpoint helper tool")
	app.Commands = []cli.Command{
		commandStressTestNormal,
		commandStressTestToken,
	}
	app.Flags = []cli.Flag{
		nodeURLFlag,
		privKeyFlag,
	}
	cli.CommandHelpTemplate = flags.OriginCommandHelpTemplate
}

// Commonly used command line flags.
var (
	nodeURLFlag = cli.StringFlag{
		Name:  "rpc",
		Value: "http://localhost:8545",
		Usage: "The rpc endpoint list of servial local or remote geth nodes(separator ',')",
	}
	privKeyFlag = cli.StringFlag{
		Name: "privkey",
		// 0x4Bee7F41037532509368b7B4CA8255b44Dd8Fb77
		Value: "14b3237e4c9a9cf5c4884cd980ed17f056bd7f09bfd08c58117d36d0dbac997e",
		Usage: "The main account used for test",
	}
	accountNumberFlag = cli.IntFlag{
		Name:  "accountNumber",
		Value: 100,
		Usage: "The number of accounts used for test",
	}
	totalTxsFlag = cli.IntFlag{
		Name:  "totalTxs",
		Value: 10000,
		Usage: "The total number of transactions sent for test",
	}
	threadsFlag = cli.IntFlag{
		Name:  "threads",
		Value: 100,
		Usage: "The go routine number for test",
	}
	tokenFlag = cli.StringFlag{
		Name:  "token",
		Value: "0x000000000000000000000000000000000000f003",
		Usage: "The token address of test",
	}
	decimalFlag = cli.IntFlag{
		Name:  "decimal",
		Value: defaultDecimal,
		Usage: "The decimal of token",
	}
)

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
	fdlimit.Raise(10000)

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
