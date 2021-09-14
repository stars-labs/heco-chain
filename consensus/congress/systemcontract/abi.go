package systemcontract

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
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
	},
	{
      "inputs": [
        {
          "internalType": "address",
          "name": "val",
          "type": "address"
        }
      ],
      "name": "getValidatorInfo",
      "outputs": [
        {
          "internalType": "address payable",
          "name": "",
          "type": "address"
        },
        {
          "internalType": "enum Validators.Status",
          "name": "",
          "type": "uint8"
        },
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        },
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        },
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        },
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        },
        {
          "internalType": "address[]",
          "name": "",
          "type": "address[]"
        }
      ],
      "stateMutability": "view",
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
        		"internalType": "uint256",
        		"name": "action",
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
				"internalType": "bytes",
				"name": "data",
				"type": "bytes"
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

const AddrListInteractiveABI = `
[
	{
	  "inputs": [],
	  "name": "blackLastUpdatedNumber",
	  "outputs": [
		{
		  "internalType": "uint256",
		  "name": "",
		  "type": "uint256"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [],
	  "name": "devVerifyEnabled",
	  "outputs": [
		{
		  "internalType": "bool",
		  "name": "",
		  "type": "bool"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [],
	  "name": "getBlacksFrom",
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
	  "inputs": [],
	  "name": "getBlacksTo",
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
		  "internalType": "uint32",
		  "name": "i",
		  "type": "uint32"
		}
	  ],
	  "name": "getRuleByIndex",
	  "outputs": [
		{
		  "internalType": "bytes32",
		  "name": "",
		  "type": "bytes32"
		},
		{
		  "internalType": "uint128",
		  "name": "",
		  "type": "uint128"
		},
		{
		  "internalType": "enum AddressList.CheckType",
		  "name": "",
		  "type": "uint8"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [],
	  "name": "initializeV2",
	  "outputs": [],
	  "stateMutability": "nonpayable",
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
	},
	{
	  "inputs": [
		{
		  "internalType": "address",
		  "name": "addr",
		  "type": "address"
		}
	  ],
	  "name": "isDeveloper",
	  "outputs": [
		{
		  "internalType": "bool",
		  "name": "",
		  "type": "bool"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [],
	  "name": "rulesLastUpdatedNumber",
	  "outputs": [
		{
		  "internalType": "uint256",
		  "name": "",
		  "type": "uint256"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [],
	  "name": "rulesLen",
	  "outputs": [
		{
		  "internalType": "uint32",
		  "name": "",
		  "type": "uint32"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	}
]`

const ValidatorsV1InteractiveABI = `[
    {
        "inputs": [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "name": "activeValidators",
        "outputs": [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
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
                "name": "_candidates",
                "type": "address[]"
            },
            {
                "internalType": "address[]",
                "name": "_manager",
                "type": "address[]"
            },
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
]`

const PunishV1InteractiveABI = `[
    {
      "inputs": [],
      "name": "initialize",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    }
]`

// DevMappingPosition is the position of the state variable `devs`.
// Since the state variables are as follow:
//    bool public initialized;
//    bool public devVerifyEnabled;
//    address public admin;
//    address public pendingAdmin;
//
//    mapping(address => bool) private devs;
//
//    //NOTE: make sure this list is not too large!
//    address[] blacksFrom;
//    address[] blacksTo;
//    mapping(address => uint256) blacksFromMap;      // address => index+1
//    mapping(address => uint256) blacksToMap;        // address => index+1
//
//    uint256 public blackLastUpdatedNumber; // last block number when the black list is updated
//    uint256 public rulesLastUpdatedNumber;  // last block number when the rules are updated
//    // event check rules
//    EventCheckRule[] rules;
//    mapping(bytes32 => mapping(uint128 => uint256)) rulesMap;   // eventSig => checkIdx => indexInArray+1
//
// according to [Layout of State Variables in Storage](https://docs.soliditylang.org/en/v0.8.4/internals/layout_in_storage.html),
// and after optimizer enabled, the `initialized`, `enabled` and `admin` will be packed, and stores at slot 0,
// `pendingAdmin` stores at slot 1, so the position for `devs` is 2.
const DevMappingPosition = 2

var (
	BlackLastUpdatedNumberPosition = common.BytesToHash([]byte{0x07})
	RulesLastUpdatedNumberPosition = common.BytesToHash([]byte{0x08})
)

var (
	ValidatorsContractName   = "validators"
	PunishContractName       = "punish"
	ProposalContractName     = "proposal"
	SysGovContractName       = "governance"
	AddressListContractName  = "address_list"
	ValidatorsV1ContractName = "validators_v1"
	PunishV1ContractName     = "punish_v1"
	ValidatorsContractAddr   = common.HexToAddress("0x000000000000000000000000000000000000f000")
	PunishContractAddr       = common.HexToAddress("0x000000000000000000000000000000000000f001")
	ProposalAddr             = common.HexToAddress("0x000000000000000000000000000000000000f002")
	SysGovContractAddr       = common.HexToAddress("0x000000000000000000000000000000000000F003")
	AddressListContractAddr  = common.HexToAddress("0x000000000000000000000000000000000000F004")
	ValidatorsV1ContractAddr = common.HexToAddress("0x000000000000000000000000000000000000F005")
	PunishV1ContractAddr     = common.HexToAddress("0x000000000000000000000000000000000000F006")
	// SysGovToAddr is the To address for the system governance transaction, NOT contract address
	SysGovToAddr = common.HexToAddress("0x000000000000000000000000000000000000ffff")

	abiMap map[string]abi.ABI
)

func init() {
	abiMap = make(map[string]abi.ABI, 0)
	tmpABI, _ := abi.JSON(strings.NewReader(ValidatorsInteractiveABI))
	abiMap[ValidatorsContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(PunishInteractiveABI))
	abiMap[PunishContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(ProposalInteractiveABI))
	abiMap[ProposalContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(SysGovInteractiveABI))
	abiMap[SysGovContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(AddrListInteractiveABI))
	abiMap[AddressListContractName] = tmpABI

	tmpABI, _ = abi.JSON(strings.NewReader(ValidatorsV1InteractiveABI))
	abiMap[ValidatorsV1ContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(PunishV1InteractiveABI))
	abiMap[PunishV1ContractName] = tmpABI
}

func GetInteractiveABI() map[string]abi.ABI {
	return abiMap
}

func GetValidatorAddr(blockNum *big.Int, config *params.ChainConfig) *common.Address {
	if config.IsRedCoast(blockNum) {
		return &ValidatorsV1ContractAddr
	}
	return &ValidatorsContractAddr
}

func GetPunishAddr(blockNum *big.Int, config *params.ChainConfig) *common.Address {
	if config.IsRedCoast(blockNum) {
		return &PunishV1ContractAddr
	}
	return &PunishContractAddr
}
