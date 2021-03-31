package systemcontract

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"strings"
)

// ValidatorsInteractiveABI contains all methods to interactive with validator contracts.
const ValidatorsInteractiveABI = `
[
	{
		"inputs": [
		  {
			"internalType": "address[]",
			"name": "vals",
			"type": "address[]"
		  }
		],
		"name": "initialize",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "distributeBlockReward",
		"outputs": [],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getTopValidators",
		"outputs": [
		  {
			"internalType": "address[]",
			"name": "",
			"type": "address[]"
		  }
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
		  {
			"internalType": "address[]",
			"name": "newSet",
			"type": "address[]"
		  },
		  {
			"internalType": "uint256",
			"name": "epoch",
			"type": "uint256"
		  }
		],
		"name": "updateActiveValidatorSet",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]
`

const PunishInteractiveABI = `
[
	{
		"inputs": [],
		"name": "initialize",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
		  {
			"internalType": "address",
			"name": "val",
			"type": "address"
		  }
		],
		"name": "punish",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
		  {
			"internalType": "uint256",
			"name": "epoch",
			"type": "uint256"
		  }
		],
		"name": "decreaseMissedBlocksCounter",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	  }
]
`

const ProposalInteractiveABI = `
[
	{
		"inputs": [
		  {
			"internalType": "address[]",
			"name": "vals",
			"type": "address[]"
		  }
		],
		"name": "initialize",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]
`

const SysGovInteractiveABI = `
[
    {
		"inputs": [
			{
				"internalType": "uint256",
				"name": "id",
				"type": "uint256"
			}
		],
		"name": "finishProposalById",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint32",
				"name": "index",
				"type": "uint32"
			}
		],
		"name": "getPassedProposalByIndex",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "id",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "from",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "value",
				"type": "uint256"
			},
			{
				"internalType": "bytes1[]",
				"name": "data",
				"type": "bytes1[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getPassedProposalCount",
		"outputs": [
			{
				"internalType": "uint32",
				"name": "",
				"type": "uint32"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "_admin",
				"type": "address"
			}
		],
		"name": "initialize",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

var (
	ValidatorsContractName = "validators"
	PunishContractName     = "punish"
	ProposalContractName   = "proposal"
	SysGovContractName     = "governance"
	ValidatorsContractAddr = common.HexToAddress("0x000000000000000000000000000000000000f000")
	PunishContractAddr     = common.HexToAddress("0x000000000000000000000000000000000000f001")
	ProposalAddr           = common.HexToAddress("0x000000000000000000000000000000000000f002")
	SysGovContractAddr     = common.HexToAddress("0x000000000000000000000000000000000000f003")
	SysGovToAddr           = common.HexToAddress("0x000000000000000000000000000000000000ffff")
)

func GetInteractiveABI() map[string]abi.ABI {
	abiMap := make(map[string]abi.ABI, 0)
	tmpABI, _ := abi.JSON(strings.NewReader(ValidatorsInteractiveABI))
	abiMap[ValidatorsContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(PunishInteractiveABI))
	abiMap[PunishContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(ProposalInteractiveABI))
	abiMap[ProposalContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(SysGovInteractiveABI))
	abiMap[SysGovContractName] = tmpABI

	return abiMap
}