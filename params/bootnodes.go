// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

import "github.com/ethereum/go-ethereum/common"

// MainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the main Ethereum network.
var MainnetBootnodes = []string{
	// Ethereum Foundation Go Bootnodes
	"enode://7bed18c87054f807bc9096501bc78f737363f357af831791bab07c4fa6c5a1a67cdcf0a097dc2cc918262ef04fb1c05c26026df5c11a6a56666f9b1fb4072210@18.178.30.66:32668",
	"enode://d67251dd3b050e555679a8abdc427a4c78a9bae174f2fd3b9163c364d27b6a69688ee067cd3214e8ceb71e6e602fd812797b085ae37ed3bf93b78e2b77ae3306@18.181.40.7:32668",
	"enode://f88bb1f5d0e42cf75ec879212b7c8477d605315d5296fba02bc4600eccf73c64427de46567a320d00985d5bc612168817ba6dff169bd6a4774e112e6db0ff6a2@18.176.66.118:32668",
	"enode://b7afda8f726b734fc487d820c13342f6ad539f34f20a11f2d667270f46672c56a4916f6093ac309c3049731ee37780e2dbbfca5e4fde5cf76681d8498da9f102@3.114.17.176:32668",
	"enode://dde97f59464fe33995c038e40b27ac7312e7c3d4516fb29cafc0e697856546005f6e6324a8791df8b0cab983557df5a60a99a26c9b4f4e58b43e90b11fb3c16e@35.74.47.19:32668",
	"enode://1029c976e2e1fa33298cfa853e8e0e7c48a48d7c6d2af3980632f5f4f0d012e569fe6de970c3f30859236b56851b6a9d5b6dc81dba5617a901536da4ca2c654a@54.249.170.71:32668",
	"enode://3a1db5370d9517ac9b0db44a2e363dc167bd54e52d65c380202649c4e269f3907bae7b1c795aa83d9b91e42b9d959af48ee7ef29f674dc171517f8b0f16023fb@35.73.75.175:32668",
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the
var TestnetBootnodes = []string{
	"enode://924543a43d18bc5759a8bdcd17fa9c7c35df63968e9333640b80b58dab94b17a012371c9d46bed10ce7508a607cac76828ca04685893958eee44ade83b856dc2@47.242.237.63:32668",
	"enode://ebad898d980b520ef6adb54ffb6a68117686e7332f1ea01f7551b7a296a34dd945445a078d7cad019d864c5ef0e0b7f2b5777d94f93adf7dc59f798af72609ac@47.242.235.121:32668",
}

var V5Bootnodes = []string{}

// KnownDNSNetwork returns the address of a public DNS-based node list for the given
// genesis hash and protocol. See https://github.com/ethereum/discv4-dns-lists for more
// information.
func KnownDNSNetwork(genesis common.Hash, protocol string) string {
	return ""
}
