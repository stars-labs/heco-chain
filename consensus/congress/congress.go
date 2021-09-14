// Copyright 2017 The go-ethereum Authors
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

// Package congress implements the proof-of-stake-authority consensus engine.
package congress

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/congress/systemcontract"
	"github.com/ethereum/go-ethereum/consensus/congress/vmcaller"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/crypto/sha3"
)

const (
	checkpointInterval = 1024 // Number of blocks after which to save the vote snapshot to the database
	inmemorySnapshots  = 128  // Number of recent vote snapshots to keep in memory
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory

	wiggleTime    = 500 * time.Millisecond // Random delay (per validator) to allow concurrent validators
	maxValidators = 21                     // Max validators allowed to seal.

	inmemoryBlacklist = 21 // Number of recent blacklist snapshots to keep in memory
)

type blacklistDirection uint

const (
	DirectionFrom blacklistDirection = iota
	DirectionTo
	DirectionBoth
)

// Congress proof-of-stake-authority protocol constants.
var (
	epochLength = uint64(30000) // Default number of blocks after which to checkpoint and reset the pending votes

	extraVanity = 32                     // Fixed number of extra-data prefix bytes reserved for validator vanity
	extraSeal   = crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for validator seal

	uncleHash = types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.

	diffInTurn = big.NewInt(2) // Block difficulty for in-turn signatures
	diffNoTurn = big.NewInt(1) // Block difficulty for out-of-turn signatures
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	// errUnknownBlock is returned when the list of validators is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")

	// errMissingVanity is returned if a block's extra-data section is shorter than
	// 32 bytes, which is required to store the validator vanity.
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")

	// errMissingSignature is returned if a block's extra-data section doesn't seem
	// to contain a 65 byte secp256k1 signature.
	errMissingSignature = errors.New("extra-data 65 byte signature suffix missing")

	// errExtraValidators is returned if non-checkpoint block contain validator data in
	// their extra-data fields.
	errExtraValidators = errors.New("non-checkpoint block contains extra validator list")

	// errInvalidExtraValidators is returned if validator data in extra-data field is invalid.
	errInvalidExtraValidators = errors.New("Invalid extra validators in extra data field")

	// errInvalidCheckpointValidators is returned if a checkpoint block contains an
	// invalid list of validators (i.e. non divisible by 20 bytes).
	errInvalidCheckpointValidators = errors.New("invalid validator list on checkpoint block")

	// errMismatchingCheckpointValidators is returned if a checkpoint block contains a
	// list of validators different than the one the local node calculated.
	errMismatchingCheckpointValidators = errors.New("mismatching validator list on checkpoint block")

	// errInvalidMixDigest is returned if a block's mix digest is non-zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")

	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash = errors.New("non empty uncle hash")

	// errInvalidDifficulty is returned if the difficulty of a block neither 1 or 2.
	errInvalidDifficulty = errors.New("invalid difficulty")

	// errWrongDifficulty is returned if the difficulty of a block doesn't match the
	// turn of the validator.
	errWrongDifficulty = errors.New("wrong difficulty")

	// errInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	errInvalidTimestamp = errors.New("invalid timestamp")

	// ErrInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	ErrInvalidTimestamp = errors.New("invalid timestamp")

	// errInvalidVotingChain is returned if an authorization list is attempted to
	// be modified via out-of-range or non-contiguous headers.
	errInvalidVotingChain = errors.New("invalid voting chain")

	// errUnauthorizedValidator is returned if a header is signed by a non-authorized entity.
	errUnauthorizedValidator = errors.New("unauthorized validator")

	// errRecentlySigned is returned if a header is signed by an authorized entity
	// that already signed a header recently, thus is temporarily not allowed to.
	errRecentlySigned = errors.New("recently signed")

	// errInvalidValidatorLen is returned if validators length is zero or bigger than maxValidators.
	errInvalidValidatorsLength = errors.New("Invalid validators length")

	// errInvalidCoinbase is returned if the coinbase isn't the validator of the block.
	errInvalidCoinbase = errors.New("Invalid coin base")

	errInvalidSysGovCount = errors.New("invalid system governance tx count")
)

var (
	getblacklistTimer = metrics.NewRegisteredTimer("congress/blacklist/get", nil)
	getRulesTimer     = metrics.NewRegisteredTimer("congress/eventcheckrules/get", nil)
)

// StateFn gets state by the state root hash.
type StateFn func(hash common.Hash) (*state.StateDB, error)

// ValidatorFn hashes and signs the data to be signed by a backing account.
type ValidatorFn func(validator accounts.Account, mimeType string, message []byte) ([]byte, error)
type SignTxFn func(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
	// If the signature's already cached, return that
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]

	// Recover the public key and the Ethereum address
	pubkey, err := crypto.Ecrecover(SealHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var validator common.Address
	copy(validator[:], crypto.Keccak256(pubkey[1:])[12:])

	sigcache.Add(hash, validator)
	return validator, nil
}

// Congress is the proof-of-stake-authority consensus engine proposed to support the
// Ethereum testnet following the Ropsten attacks.
type Congress struct {
	chainConfig *params.ChainConfig    // ChainConfig to execute evm
	config      *params.CongressConfig // Consensus engine configuration parameters
	db          ethdb.Database         // Database to store and retrieve snapshot checkpoints

	recents    *lru.ARCCache // Snapshots for recent block to speed up reorgs
	signatures *lru.ARCCache // Signatures of recent blocks to speed up mining

	blacklists      *lru.Cache // blacklists caches recent blacklist to speed up transactions validation
	blLock          sync.Mutex // Make sure only get blacklist once for each block
	eventCheckRules *lru.Cache // eventCheckRules caches recent EventCheckRules to speed up log validation
	rulesLock       sync.Mutex // Make sure only get eventCheckRules once for each block

	proposals map[common.Address]bool // Current list of proposals we are pushing

	signer types.Signer // the signer instance to recover tx sender

	validator common.Address // Ethereum address of the signing key
	signFn    ValidatorFn    // Validator function to authorize hashes with
	signTxFn  SignTxFn
	lock      sync.RWMutex // Protects the validator fields

	stateFn StateFn // Function to get state by state root

	abi map[string]abi.ABI // Interactive with system contracts

	chain consensus.ChainHeaderReader // chain is only for reading parent headers when getting blacklist and rules

	// The fields below are for testing only
	fakeDiff bool // Skip difficulty verifications
}

// New creates a Congress proof-of-stake-authority consensus engine with the initial
// validators set to the ones provided by the user.
func New(chainConfig *params.ChainConfig, db ethdb.Database) *Congress {
	// Set any missing consensus parameters to their defaults
	conf := *chainConfig.Congress
	if conf.Epoch == 0 {
		conf.Epoch = epochLength
	}
	// Allocate the snapshot caches and create the engine
	recents, _ := lru.NewARC(inmemorySnapshots)
	signatures, _ := lru.NewARC(inmemorySignatures)
	blacklists, _ := lru.New(inmemoryBlacklist)
	rules, _ := lru.New(inmemoryBlacklist)

	abi := systemcontract.GetInteractiveABI()

	return &Congress{
		chainConfig:     chainConfig,
		config:          &conf,
		db:              db,
		recents:         recents,
		signatures:      signatures,
		blacklists:      blacklists,
		eventCheckRules: rules,
		proposals:       make(map[common.Address]bool),
		abi:             abi,
		signer:          types.LatestSignerForChainID(chainConfig.ChainID),
	}
}

func (c *Congress) SetChain(chain consensus.ChainHeaderReader) {
	c.chain = chain
}

// SetStateFn sets the function to get state.
func (c *Congress) SetStateFn(fn StateFn) {
	c.stateFn = fn
}

// Author implements consensus.Engine, returning the Ethereum address recovered
// from the signature in the header's extra-data section.
func (c *Congress) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
	// return ecrecover(header, c.signatures)
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *Congress) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return c.verifyHeader(chain, header, nil)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers. The
// method returns a quit channel to abort the operations and a results channel to
// retrieve the async verifications (the order is that of the input slice).
func (c *Congress) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := c.verifyHeader(chain, header, headers[:i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (c *Congress) verifyHeader(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}
	number := header.Number.Uint64()

	// Don't waste time checking blocks from the future
	if header.Time > uint64(time.Now().Unix()) {
		return consensus.ErrFutureBlock
	}
	// Check that the extra-data contains the vanity, validators and signature.
	if len(header.Extra) < extraVanity {
		return errMissingVanity
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errMissingSignature
	}
	// check extra data
	isEpoch := number%c.config.Epoch == 0

	// Ensure that the extra-data contains a validator list on checkpoint, but none otherwise
	validatorsBytes := len(header.Extra) - extraVanity - extraSeal
	if !isEpoch && validatorsBytes != 0 {
		return errExtraValidators
	}
	// Ensure that the validator bytes length is valid
	if isEpoch && validatorsBytes%common.AddressLength != 0 {
		return errExtraValidators
	}

	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in PoA
	if header.UncleHash != uncleHash {
		return errInvalidUncleHash
	}
	// Ensure that the block's difficulty is meaningful (may not be correct at this point)
	if number > 0 && header.Difficulty == nil {
		return errInvalidDifficulty
	}
	// Verify that the gas limit is <= 2^63-1
	cap := uint64(0x7fffffffffffffff)
	if header.GasLimit > cap {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, cap)
	}
	// If all checks passed, validate any special fields for hard forks
	if err := misc.VerifyForkHashes(chain.Config(), header, false); err != nil {
		return err
	}
	// All basic checks passed, verify cascading fields
	return c.verifyCascadingFields(chain, header, parents)
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (c *Congress) verifyCascadingFields(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	// The genesis block is the always valid dead-end
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}

	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}

	if parent.Time+c.config.Period > header.Time {
		return ErrInvalidTimestamp
	}

	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}
	if !chain.Config().IsLondon(header.Number) {
		// Verify BaseFee not present before EIP-1559 fork.
		if header.BaseFee != nil {
			return fmt.Errorf("invalid baseFee before fork: have %d, want <nil>", header.BaseFee)
		}
		if err := misc.VerifyGaslimit(parent.GasLimit, header.GasLimit); err != nil {
			return err
		}
	} else if err := misc.VerifyEip1559Header(chain.Config(), parent, header); err != nil {
		// Verify the header's EIP-1559 attributes.
		return err
	}

	// All basic checks passed, verify the seal and return
	return c.verifySeal(chain, header, parents)
}

// snapshot retrieves the authorization snapshot at a given point in time.
func (c *Congress) snapshot(chain consensus.ChainHeaderReader, number uint64, hash common.Hash, parents []*types.Header) (*Snapshot, error) {
	// Search for a snapshot in memory or on disk for checkpoints
	var (
		headers []*types.Header
		snap    *Snapshot
	)
	for snap == nil {
		// If an in-memory snapshot was found, use that
		if s, ok := c.recents.Get(hash); ok {
			snap = s.(*Snapshot)
			break
		}
		// If an on-disk checkpoint snapshot can be found, use that
		if number%checkpointInterval == 0 {
			if s, err := loadSnapshot(c.config, c.signatures, c.db, hash); err == nil {
				log.Trace("Loaded voting snapshot from disk", "number", number, "hash", hash)
				snap = s
				break
			}
		}
		// If we're at the genesis, snapshot the initial state. Alternatively if we're
		// at a checkpoint block without a parent (light client CHT), or we have piled
		// up more headers than allowed to be reorged (chain reinit from a freezer),
		// consider the checkpoint trusted and snapshot it.
		if number == 0 || (number%c.config.Epoch == 0 && (len(headers) > params.FullImmutabilityThreshold || chain.GetHeaderByNumber(number-1) == nil)) {
			checkpoint := chain.GetHeaderByNumber(number)
			if checkpoint != nil {
				hash := checkpoint.Hash()

				validators := make([]common.Address, (len(checkpoint.Extra)-extraVanity-extraSeal)/common.AddressLength)
				for i := 0; i < len(validators); i++ {
					copy(validators[i][:], checkpoint.Extra[extraVanity+i*common.AddressLength:])
				}
				snap = newSnapshot(c.config, c.signatures, number, hash, validators)
				if err := snap.store(c.db); err != nil {
					return nil, err
				}
				log.Info("Stored checkpoint snapshot to disk", "number", number, "hash", hash)
				break
			}
		}
		// No snapshot for this header, gather the header and move backward
		var header *types.Header
		if len(parents) > 0 {
			// If we have explicit parents, pick from there (enforced)
			header = parents[len(parents)-1]
			if header.Hash() != hash || header.Number.Uint64() != number {
				return nil, consensus.ErrUnknownAncestor
			}
			parents = parents[:len(parents)-1]
		} else {
			// No explicit parents (or no more left), reach out to the database
			header = chain.GetHeader(hash, number)
			if header == nil {
				return nil, consensus.ErrUnknownAncestor
			}
		}
		headers = append(headers, header)
		number, hash = number-1, header.ParentHash
	}
	// Previous snapshot found, apply any pending headers on top of it
	for i := 0; i < len(headers)/2; i++ {
		headers[i], headers[len(headers)-1-i] = headers[len(headers)-1-i], headers[i]
	}
	snap, err := snap.apply(headers, chain, parents)
	if err != nil {
		return nil, err
	}
	c.recents.Add(snap.Hash, snap)

	// If we've generated a new checkpoint snapshot, save to disk
	if snap.Number%checkpointInterval == 0 && len(headers) > 0 {
		if err = snap.store(c.db); err != nil {
			return nil, err
		}
		log.Trace("Stored voting snapshot to disk", "number", snap.Number, "hash", snap.Hash)
	}
	return snap, err
}

// VerifyUncles implements consensus.Engine, always returning an error for any
// uncles as this consensus mechanism doesn't permit uncles.
func (c *Congress) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

// VerifySeal implements consensus.Engine, checking whether the signature contained
// in the header satisfies the consensus protocol requirements.
func (c *Congress) VerifySeal(chain consensus.ChainHeaderReader, header *types.Header) error {
	return c.verifySeal(chain, header, nil)
}

// verifySeal checks whether the signature contained in the header satisfies the
// consensus protocol requirements. The method accepts an optional list of parent
// headers that aren't yet part of the local blockchain to generate the snapshots
// from.
func (c *Congress) verifySeal(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	// Verifying the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
	// Retrieve the snapshot needed to verify this header and cache it
	snap, err := c.snapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return err
	}

	// Resolve the authorization key and check against validators
	signer, err := ecrecover(header, c.signatures)
	if err != nil {
		return err
	}
	if signer != header.Coinbase {
		return errInvalidCoinbase
	}

	if _, ok := snap.Validators[signer]; !ok {
		return errUnauthorizedValidator
	}

	for seen, recent := range snap.Recents {
		if recent == signer {
			// Validator is among recents, only fail if the current block doesn't shift it out
			if limit := uint64(len(snap.Validators)/2 + 1); seen > number-limit {
				return errRecentlySigned
			}
		}
	}

	// Ensure that the difficulty corresponds to the turn-ness of the signer
	if !c.fakeDiff {
		inturn := snap.inturn(header.Number.Uint64(), signer)
		if inturn && header.Difficulty.Cmp(diffInTurn) != 0 {
			return errWrongDifficulty
		}
		if !inturn && header.Difficulty.Cmp(diffNoTurn) != 0 {
			return errWrongDifficulty
		}
	}

	return nil
}

// Prepare implements consensus.Engine, preparing all the consensus fields of the
// header for running the transactions on top.
func (c *Congress) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	// If the block isn't a checkpoint, cast a random vote (good enough for now)
	header.Coinbase = c.validator
	header.Nonce = types.BlockNonce{}

	number := header.Number.Uint64()
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}

	// Set the correct difficulty
	header.Difficulty = calcDifficulty(snap, c.validator)

	// Ensure the extra data has all its components
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]

	if number%c.config.Epoch == 0 {
		newSortedValidators, err := c.getTopValidators(chain, header)
		if err != nil {
			return err
		}

		for _, validator := range newSortedValidators {
			header.Extra = append(header.Extra, validator.Bytes()...)
		}
	}
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)

	// Mix digest is reserved for now, set to empty
	header.MixDigest = common.Hash{}

	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = parent.Time + c.config.Period
	if header.Time < uint64(time.Now().Unix()) {
		header.Time = uint64(time.Now().Unix())
	}
	return nil
}

// Finalize implements consensus.Engine, ensuring no uncles are set, nor block
// rewards given.
func (c *Congress) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs *[]*types.Transaction, uncles []*types.Header, receipts *[]*types.Receipt, systemTxs []*types.Transaction) error {
	// Initialize all system contracts at block 1.
	if header.Number.Cmp(common.Big1) == 0 {
		if err := c.initializeSystemContracts(chain, header, state); err != nil {
			log.Error("Initialize system contracts failed", "err", err)
			return err
		}
	}

	if header.Difficulty.Cmp(diffInTurn) != 0 {
		if err := c.tryPunishValidator(chain, header, state); err != nil {
			return err
		}
	}

	// avoid nil pointer
	if txs == nil {
		s := make([]*types.Transaction, 0)
		txs = &s
	}
	if receipts == nil {
		rs := make([]*types.Receipt, 0)
		receipts = &rs
	}

	// execute block reward tx.
	if len(*txs) > 0 {
		if err := c.trySendBlockReward(chain, header, state); err != nil {
			return err
		}
	}

	// do epoch thing at the end, because it will update active validators
	if header.Number.Uint64()%c.config.Epoch == 0 {
		newValidators, err := c.doSomethingAtEpoch(chain, header, state)
		if err != nil {
			return err
		}

		validatorsBytes := make([]byte, len(newValidators)*common.AddressLength)
		for i, validator := range newValidators {
			copy(validatorsBytes[i*common.AddressLength:], validator.Bytes())
		}

		extraSuffix := len(header.Extra) - extraSeal
		if !bytes.Equal(header.Extra[extraVanity:extraSuffix], validatorsBytes) {
			return errInvalidExtraValidators
		}
	}

	//handle system governance Proposal
	if chain.Config().IsRedCoast(header.Number) {
		proposalCount, err := c.getPassedProposalCount(chain, header, state)
		if err != nil {
			return err
		}
		if proposalCount != uint32(len(systemTxs)) {
			return errInvalidSysGovCount
		}
		// Due to the logics of the finish operation of contract `governance`, when finishing a proposal which
		// is not the last passed proposal, it will change the sequence. So in here we must first executes all
		// passed proposals, and then finish then all.
		pIds := make([]*big.Int, 0, proposalCount)
		for i := uint32(0); i < proposalCount; i++ {
			prop, err := c.getPassedProposalByIndex(chain, header, state, i)
			if err != nil {
				return err
			}
			// execute the system governance Proposal
			tx := systemTxs[int(i)]
			receipt, err := c.replayProposal(chain, header, state, prop, len(*txs), tx)
			if err != nil {
				return err
			}
			*txs = append(*txs, tx)
			*receipts = append(*receipts, receipt)
			// set
			pIds = append(pIds, prop.Id)
		}
		// Finish all proposal
		for i := uint32(0); i < proposalCount; i++ {
			err = c.finishProposalById(chain, header, state, pIds[i])
			if err != nil {
				return err
			}
		}
	}

	// No block rewards in PoA, so the state remains as is and uncles are dropped
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.CalcUncleHash(nil)

	return nil
}

// FinalizeAndAssemble implements consensus.Engine, ensuring no uncles are set,
// nor block rewards given, and returns the final block.
func (c *Congress) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (b *types.Block, rs []*types.Receipt, err error) {
	defer func() {
		if err != nil {
			log.Warn("FinalizeAndAssemble failed", "err", err)
		}
	}()
	// Initialize all system contracts at block 1.
	if header.Number.Cmp(common.Big1) == 0 {
		if err := c.initializeSystemContracts(chain, header, state); err != nil {
			panic(err)
		}
	}

	// punish validator if necessary
	if header.Difficulty.Cmp(diffInTurn) != 0 {
		if err := c.tryPunishValidator(chain, header, state); err != nil {
			panic(err)
		}
	}

	// deposit block reward if any tx exists.
	if len(txs) > 0 {
		if err := c.trySendBlockReward(chain, header, state); err != nil {
			panic(err)
		}
	}

	// do epoch thing at the end, because it will update active validators
	if header.Number.Uint64()%c.config.Epoch == 0 {
		if _, err := c.doSomethingAtEpoch(chain, header, state); err != nil {
			panic(err)
		}
	}

	//handle system governance Proposal
	//
	// Note:
	// Even if the miner is not `running`, it's still working,
	// the 'miner.worker' will try to FinalizeAndAssemble a block,
	// in this case, the signTxFn is not set. A `non-miner node` can't execute system governance proposal.
	if c.signTxFn != nil && chain.Config().IsRedCoast(header.Number) {
		proposalCount, err := c.getPassedProposalCount(chain, header, state)
		if err != nil {
			return nil, nil, err
		}

		// Due to the logics of the finish operation of contract `governance`, when finishing a proposal which
		// is not the last passed proposal, it will change the sequence. So in here we must first executes all
		// passed proposals, and then finish then all.
		pIds := make([]*big.Int, 0, proposalCount)
		for i := uint32(0); i < proposalCount; i++ {
			prop, err := c.getPassedProposalByIndex(chain, header, state, i)
			if err != nil {
				return nil, nil, err
			}
			// execute the system governance Proposal
			tx, receipt, err := c.executeProposal(chain, header, state, prop, len(txs))
			if err != nil {
				return nil, nil, err
			}
			txs = append(txs, tx)
			receipts = append(receipts, receipt)
			// set
			pIds = append(pIds, prop.Id)
		}
		// Finish all proposal
		for i := uint32(0); i < proposalCount; i++ {
			err = c.finishProposalById(chain, header, state, pIds[i])
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// No block rewards in PoA, so the state remains as is and uncles are dropped
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.CalcUncleHash(nil)

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, nil, receipts, new(trie.Trie)), receipts, nil
}

func (c *Congress) trySendBlockReward(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) error {
	fee := state.GetBalance(consensus.FeeRecoder)
	if fee.Cmp(common.Big0) <= 0 {
		return nil
	}

	// Miner will send tx to deposit block fees to contract, add to his balance first.
	state.AddBalance(header.Coinbase, fee)
	// reset fee
	state.SetBalance(consensus.FeeRecoder, common.Big0)

	method := "distributeBlockReward"
	data, err := c.abi[systemcontract.ValidatorsContractName].Pack(method)
	if err != nil {
		log.Error("Can't pack data for distributeBlockReward", "err", err)
		return err
	}

	nonce := state.GetNonce(header.Coinbase)
	msg := vmcaller.NewLegacyMessage(header.Coinbase, systemcontract.GetValidatorAddr(header.Number, c.chainConfig), nonce, fee, math.MaxUint64, new(big.Int), data, true)

	if _, err := vmcaller.ExecuteMsg(msg, state, header, newChainContext(chain, c), c.chainConfig); err != nil {
		return err
	}

	return nil
}

func (c *Congress) tryPunishValidator(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) error {
	number := header.Number.Uint64()
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	validators := snap.validators()
	outTurnValidator := validators[number%uint64(len(validators))]
	// check sigend recently or not
	signedRecently := false
	for _, recent := range snap.Recents {
		if recent == outTurnValidator {
			signedRecently = true
			break
		}
	}
	if !signedRecently {
		if err := c.punishValidator(outTurnValidator, chain, header, state); err != nil {
			return err
		}
	}

	return nil
}

func (c *Congress) doSomethingAtEpoch(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) ([]common.Address, error) {
	newSortedValidators, err := c.getTopValidators(chain, header)
	if err != nil {
		return []common.Address{}, err
	}

	// update contract new validators if new set exists
	if err := c.updateValidators(newSortedValidators, chain, header, state); err != nil {
		return []common.Address{}, err
	}
	//  decrease validator missed blocks counter at epoch
	if err := c.decreaseMissedBlocksCounter(chain, header, state); err != nil {
		return []common.Address{}, err
	}

	return newSortedValidators, nil
}

// initializeSystemContracts initializes all genesis system contracts.
func (c *Congress) initializeSystemContracts(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) error {
	snap, err := c.snapshot(chain, 0, header.ParentHash, nil)
	if err != nil {
		return err
	}

	genesisValidators := snap.validators()
	if len(genesisValidators) == 0 || len(genesisValidators) > maxValidators {
		return errInvalidValidatorsLength
	}

	method := "initialize"
	contracts := []struct {
		addr    common.Address
		packFun func() ([]byte, error)
	}{
		{systemcontract.ValidatorsContractAddr, func() ([]byte, error) {
			return c.abi[systemcontract.ValidatorsContractName].Pack(method, genesisValidators)
		}},
		{systemcontract.PunishContractAddr, func() ([]byte, error) { return c.abi[systemcontract.PunishContractName].Pack(method) }},
		{systemcontract.ProposalAddr, func() ([]byte, error) {
			return c.abi[systemcontract.ProposalContractName].Pack(method, genesisValidators)
		}},
	}

	for _, contract := range contracts {
		data, err := contract.packFun()
		if err != nil {
			return err
		}

		nonce := state.GetNonce(header.Coinbase)
		msg := vmcaller.NewLegacyMessage(header.Coinbase, &contract.addr, nonce, new(big.Int), math.MaxUint64, new(big.Int), data, true)

		if _, err := vmcaller.ExecuteMsg(msg, state, header, newChainContext(chain, c), c.chainConfig); err != nil {
			return err
		}
	}

	return nil
}

// call this at epoch block to get top validators based on the state of epoch block - 1
func (c *Congress) getTopValidators(chain consensus.ChainHeaderReader, header *types.Header) ([]common.Address, error) {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return []common.Address{}, consensus.ErrUnknownAncestor
	}
	statedb, err := c.stateFn(parent.Root)
	if err != nil {
		return []common.Address{}, err
	}

	method := "getTopValidators"
	data, err := c.abi[systemcontract.ValidatorsContractName].Pack(method)
	if err != nil {
		log.Error("Can't pack data for getTopValidators", "error", err)
		return []common.Address{}, err
	}

	msg := vmcaller.NewLegacyMessage(header.Coinbase, systemcontract.GetValidatorAddr(parent.Number, c.chainConfig), 0, new(big.Int), math.MaxUint64, new(big.Int), data, false)

	// use parent
	result, err := vmcaller.ExecuteMsg(msg, statedb, parent, newChainContext(chain, c), c.chainConfig)
	if err != nil {
		return []common.Address{}, err
	}

	// unpack data
	ret, err := c.abi[systemcontract.ValidatorsContractName].Unpack(method, result)
	if err != nil {
		return []common.Address{}, err
	}
	if len(ret) != 1 {
		return []common.Address{}, errors.New("Invalid params length")
	}
	validators, ok := ret[0].([]common.Address)
	if !ok {
		return []common.Address{}, errors.New("Invalid validators format")
	}
	sort.Sort(validatorsAscending(validators))
	return validators, err
}

func (c *Congress) updateValidators(vals []common.Address, chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) error {
	// method
	method := "updateActiveValidatorSet"
	data, err := c.abi[systemcontract.ValidatorsContractName].Pack(method, vals, new(big.Int).SetUint64(c.config.Epoch))
	if err != nil {
		log.Error("Can't pack data for updateActiveValidatorSet", "error", err)
		return err
	}

	// call contract
	nonce := state.GetNonce(header.Coinbase)
	msg := vmcaller.NewLegacyMessage(header.Coinbase, systemcontract.GetValidatorAddr(header.Number, c.chainConfig), nonce, new(big.Int), math.MaxUint64, new(big.Int), data, true)
	if _, err := vmcaller.ExecuteMsg(msg, state, header, newChainContext(chain, c), c.chainConfig); err != nil {
		log.Error("Can't update validators to contract", "err", err)
		return err
	}

	return nil
}

func (c *Congress) punishValidator(val common.Address, chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) error {
	// method
	method := "punish"
	data, err := c.abi[systemcontract.PunishContractName].Pack(method, val)
	if err != nil {
		log.Error("Can't pack data for punish", "error", err)
		return err
	}

	// call contract
	nonce := state.GetNonce(header.Coinbase)
	msg := vmcaller.NewLegacyMessage(header.Coinbase, systemcontract.GetPunishAddr(header.Number, c.chainConfig), nonce, new(big.Int), math.MaxUint64, new(big.Int), data, true)
	if _, err := vmcaller.ExecuteMsg(msg, state, header, newChainContext(chain, c), c.chainConfig); err != nil {
		log.Error("Can't punish validator", "err", err)
		return err
	}

	return nil
}

func (c *Congress) decreaseMissedBlocksCounter(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) error {
	// method
	method := "decreaseMissedBlocksCounter"
	data, err := c.abi[systemcontract.PunishContractName].Pack(method, new(big.Int).SetUint64(c.config.Epoch))
	if err != nil {
		log.Error("Can't pack data for decreaseMissedBlocksCounter", "error", err)
		return err
	}

	// call contract
	nonce := state.GetNonce(header.Coinbase)
	msg := vmcaller.NewLegacyMessage(header.Coinbase, systemcontract.GetPunishAddr(header.Number, c.chainConfig), nonce, new(big.Int), math.MaxUint64, new(big.Int), data, true)
	if _, err := vmcaller.ExecuteMsg(msg, state, header, newChainContext(chain, c), c.chainConfig); err != nil {
		log.Error("Can't decrease missed blocks counter for validator", "err", err)
		return err
	}

	return nil
}

// Authorize injects a private key into the consensus engine to mint new blocks
// with.
func (c *Congress) Authorize(validator common.Address, signFn ValidatorFn, signTxFn SignTxFn) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.validator = validator
	c.signFn = signFn
	c.signTxFn = signTxFn
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (c *Congress) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	// Sealing the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
	// For 0-period chains, refuse to seal empty blocks (no reward but would spin sealing)
	if c.config.Period == 0 && len(block.Transactions()) == 0 {
		log.Info("Sealing paused, waiting for transactions")
		return nil
	}
	// Don't hold the val fields for the entire sealing procedure
	c.lock.RLock()
	val, signFn := c.validator, c.signFn
	c.lock.RUnlock()

	// Bail out if we're unauthorized to sign a block
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	if _, authorized := snap.Validators[val]; !authorized {
		return errUnauthorizedValidator
	}
	// If we're amongst the recent validators, wait for the next block
	for seen, recent := range snap.Recents {
		if recent == val {
			// Validator is among recents, only wait if the current block doesn't shift it out
			if limit := uint64(len(snap.Validators)/2 + 1); number < limit || seen > number-limit {
				log.Info("Signed recently, must wait for others")
				return nil
			}
		}
	}

	// Sweet, the protocol permits us to sign the block, wait for our time
	delay := time.Unix(int64(header.Time), 0).Sub(time.Now()) // nolint: gosimple
	if header.Difficulty.Cmp(diffNoTurn) == 0 {
		// It's not our turn explicitly to sign, delay it a bit
		wiggle := time.Duration(len(snap.Validators)/2+1) * wiggleTime
		delay += time.Duration(rand.Int63n(int64(wiggle)))

		log.Trace("Out-of-turn signing requested", "wiggle", common.PrettyDuration(wiggle))
	}
	// Sign all the things!
	sighash, err := signFn(accounts.Account{Address: val}, accounts.MimetypeCongress, CongressRLP(header))
	if err != nil {
		return err
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
	// Wait until sealing is terminated or delay timeout.
	log.Trace("Waiting for slot to sign and propagate", "delay", common.PrettyDuration(delay))
	go func() {
		select {
		case <-stop:
			return
		case <-time.After(delay):
		}

		select {
		case results <- block.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", SealHash(header))
		}
	}()

	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have:
// * DIFF_NOTURN(2) if BLOCK_NUMBER % validator_COUNT != validator_INDEX
// * DIFF_INTURN(1) if BLOCK_NUMBER % validator_COUNT == validator_INDEX
func (c *Congress) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	snap, err := c.snapshot(chain, parent.Number.Uint64(), parent.Hash(), nil)
	if err != nil {
		return nil
	}
	return calcDifficulty(snap, c.validator)
}

func calcDifficulty(snap *Snapshot, validator common.Address) *big.Int {
	if snap.inturn(snap.Number+1, validator) {
		return new(big.Int).Set(diffInTurn)
	}
	return new(big.Int).Set(diffNoTurn)
}

// SealHash returns the hash of a block prior to it being sealed.
func (c *Congress) SealHash(header *types.Header) common.Hash {
	return SealHash(header)
}

// Close implements consensus.Engine. It's a noop for congress as there are no background threads.
func (c *Congress) Close() error {
	return nil
}

// APIs implements consensus.Engine, returning the user facing RPC API to allow
// controlling the validator voting.
func (c *Congress) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{{
		Namespace: "congress",
		Version:   "1.0",
		Service:   &API{chain: chain, congress: c},
		Public:    false,
	}}
}

// SealHash returns the hash of a block prior to it being sealed.
func SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.Sum(hash[:0])
	return hash
}

// CongressRLP returns the rlp bytes which needs to be signed for the proof-of-stake-authority
// sealing. The RLP to sign consists of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
//
// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func CongressRLP(header *types.Header) []byte {
	b := new(bytes.Buffer)
	encodeSigHeader(b, header)
	return b.Bytes()
}

func encodeSigHeader(w io.Writer, header *types.Header) {
	err := rlp.Encode(w, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-crypto.SignatureLength], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
	})
	if err != nil {
		panic("can't encode: " + err.Error())
	}
}

func (c *Congress) PreHandle(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) error {
	if c.chainConfig.RedCoastBlock != nil && c.chainConfig.RedCoastBlock.Cmp(header.Number) == 0 {
		return systemcontract.ApplySystemContractUpgrade(systemcontract.SysContractV1, state, header, newChainContext(chain, c), c.chainConfig)
	}
	if c.chainConfig.SophonBlock != nil && c.chainConfig.SophonBlock.Cmp(header.Number) == 0 {
		return systemcontract.ApplySystemContractUpgrade(systemcontract.SysContractV2, state, header, newChainContext(chain, c), c.chainConfig)
	}
	return nil
}

// IsSysTransaction checks whether a specific transaction is a system transaction.
func (c *Congress) IsSysTransaction(sender common.Address, tx *types.Transaction, header *types.Header) (bool, error) {
	if tx.To() == nil {
		return false, nil
	}

	to := tx.To()
	if sender == header.Coinbase && *to == systemcontract.SysGovToAddr && tx.GasPrice().Sign() == 0 {
		return true, nil
	}
	// Make sure the miner can NOT call the system contract through a normal transaction.
	if sender == header.Coinbase && *to == systemcontract.SysGovContractAddr {
		return true, nil
	}
	return false, nil
}

// CanCreate determines where a given address can create a new contract.
//
// This will queries the system Developers contract, by DIRECTLY to get the target slot value of the contract,
// it means that it's strongly relative to the layout of the Developers contract's state variables
func (c *Congress) CanCreate(state consensus.StateReader, addr common.Address, height *big.Int) bool {
	if c.chainConfig.IsRedCoast(height) && c.config.EnableDevVerification {
		if isDeveloperVerificationEnabled(state) {
			slot := calcSlotOfDevMappingKey(addr)
			valueHash := state.GetState(systemcontract.AddressListContractAddr, slot)
			// none zero value means true
			return valueHash.Big().Sign() > 0
		}
	}
	return true
}

// ValidateTx do a consensus-related validation on the given transaction at the given header and state.
// the parentState must be the state of the header's parent block.
func (c *Congress) ValidateTx(sender common.Address, tx *types.Transaction, header *types.Header, parentState *state.StateDB) error {
	// Must use the parent state for current validation,
	// so we must starting the validation after redCoastBlock
	if c.chainConfig.RedCoastBlock != nil && c.chainConfig.RedCoastBlock.Cmp(header.Number) < 0 {
		m, err := c.getBlacklist(header, parentState)
		if err != nil {
			return err
		}
		if d, exist := m[sender]; exist && (d != DirectionTo) {
			log.Trace("Hit blacklist", "tx", tx.Hash().String(), "addr", sender.String(), "direction", d)
			return types.ErrAddressDenied
		}
		if to := tx.To(); to != nil {
			if d, exist := m[*to]; exist && (d != DirectionFrom) {
				log.Trace("Hit blacklist", "tx", tx.Hash().String(), "addr", to.String(), "direction", d)
				return types.ErrAddressDenied
			}
		}
	}
	return nil
}

func (c *Congress) getBlacklist(header *types.Header, parentState *state.StateDB) (map[common.Address]blacklistDirection, error) {
	defer func(start time.Time) {
		getblacklistTimer.UpdateSince(start)
	}(time.Now())

	if v, ok := c.blacklists.Get(header.ParentHash); ok {
		return v.(map[common.Address]blacklistDirection), nil
	}

	c.blLock.Lock()
	defer c.blLock.Unlock()
	if v, ok := c.blacklists.Get(header.ParentHash); ok {
		return v.(map[common.Address]blacklistDirection), nil
	}

	// if the last updates is long ago, we don't need to get blacklist from the contract.
	if c.chainConfig.SophonBlock != nil && header.Number.Cmp(c.chainConfig.SophonBlock) > 0 {
		num := header.Number.Uint64()
		lastUpdated := lastBlacklistUpdatedNumber(parentState)
		if num >= 2 && num > lastUpdated+1 {
			parent := c.chain.GetHeader(header.ParentHash, num-1)
			if parent != nil {
				if v, ok := c.blacklists.Get(parent.ParentHash); ok {
					m := v.(map[common.Address]blacklistDirection)
					c.blacklists.Add(header.ParentHash, m)
					return m, nil
				}
			} else {
				log.Error("Unexpected error when getBlacklist, can not get parent from chain", "number", num, "blockHash", header.Hash(), "parentHash", header.ParentHash)
			}
		}
	}

	// can't get blacklist from cache, try to call the contract
	alABI := c.abi[systemcontract.AddressListContractName]
	get := func(method string) ([]common.Address, error) {
		ret, err := c.commonCallContract(header, parentState, alABI, systemcontract.AddressListContractAddr, method, 1)
		if err != nil {
			log.Error(fmt.Sprintf("%s failed", method), "err", err)
			return nil, err
		}

		blacks, ok := ret[0].([]common.Address)
		if !ok {
			return []common.Address{}, errors.New("invalid blacklist format")
		}
		return blacks, nil
	}
	froms, err := get("getBlacksFrom")
	if err != nil {
		return nil, err
	}
	tos, err := get("getBlacksTo")
	if err != nil {
		return nil, err
	}

	m := make(map[common.Address]blacklistDirection)
	for _, from := range froms {
		m[from] = DirectionFrom
	}
	for _, to := range tos {
		if _, exist := m[to]; exist {
			m[to] = DirectionBoth
		} else {
			m[to] = DirectionTo
		}
	}
	c.blacklists.Add(header.ParentHash, m)
	return m, nil
}

func (c *Congress) CreateEvmExtraValidator(header *types.Header, parentState *state.StateDB) types.EvmExtraValidator {
	if c.chainConfig.SophonBlock != nil && c.chainConfig.SophonBlock.Cmp(header.Number) < 0 {
		blacks, err := c.getBlacklist(header, parentState)
		if err != nil {
			log.Error("getBlacklist failed", "err", err)
			return nil
		}
		rules, err := c.getEventCheckRules(header, parentState)
		if err != nil {
			log.Error("getEventCheckRules failed", "err", err)
			return nil
		}
		return &blacklistValidator{
			blacks: blacks,
			rules:  rules,
		}
	}
	return nil
}

func (c *Congress) getEventCheckRules(header *types.Header, parentState *state.StateDB) (map[common.Hash]*EventCheckRule, error) {
	defer func(start time.Time) {
		getRulesTimer.UpdateSince(start)
	}(time.Now())

	if v, ok := c.eventCheckRules.Get(header.ParentHash); ok {
		return v.(map[common.Hash]*EventCheckRule), nil
	}

	c.rulesLock.Lock()
	defer c.rulesLock.Unlock()
	if v, ok := c.eventCheckRules.Get(header.ParentHash); ok {
		return v.(map[common.Hash]*EventCheckRule), nil
	}

	// if the last updates is long ago, we don't need to get blacklist from the contract.
	num := header.Number.Uint64()
	lastUpdated := lastRulesUpdatedNumber(parentState)
	if num >= 2 && num > lastUpdated+1 {
		parent := c.chain.GetHeader(header.ParentHash, num-1)
		if parent != nil {
			if v, ok := c.eventCheckRules.Get(parent.ParentHash); ok {
				m := v.(map[common.Hash]*EventCheckRule)
				c.eventCheckRules.Add(header.ParentHash, m)
				return m, nil
			}
		} else {
			log.Error("Unexpected error when getEventCheckRules, can not get parent from chain", "number", num, "blockHash", header.Hash(), "parentHash", header.ParentHash)
		}
	}

	// can't get blacklist from cache, try to call the contract
	alABI := c.abi[systemcontract.AddressListContractName]
	method := "getRuleByIndex"
	get := func(i uint32) (common.Hash, int, common.AddressCheckType, error) {
		ret, err := c.commonCallContract(header, parentState, alABI, systemcontract.AddressListContractAddr, method, 3, i)
		if err != nil {
			return common.Hash{}, 0, common.CheckNone, err
		}
		sig := ret[0].([32]byte)
		idx := ret[1].(*big.Int).Uint64()
		ct := ret[2].(uint8)

		return sig, int(idx), common.AddressCheckType(ct), nil
	}

	cnt, err := c.getEventCheckRulesLen(header, parentState)
	if err != nil {
		log.Error("getEventCheckRulesLen failed", "err", err)
		return nil, err
	}
	rules := make(map[common.Hash]*EventCheckRule)
	for i := 0; i < cnt; i++ {
		sig, idx, ct, err := get(uint32(i))
		if err != nil {
			log.Error("getRuleByIndex failed", "index", i, "number", num, "blockHash", header.Hash(), "err", err)
			return nil, err
		}
		rule, exist := rules[sig]
		if !exist {
			rule = &EventCheckRule{
				EventSig: sig,
				Checks:   make(map[int]common.AddressCheckType),
			}
			rules[sig] = rule
		}
		rule.Checks[idx] = ct
	}

	c.eventCheckRules.Add(header.ParentHash, rules)
	return rules, nil
}

func (c *Congress) getEventCheckRulesLen(header *types.Header, parentState *state.StateDB) (int, error) {
	ret, err := c.commonCallContract(header, parentState, c.abi[systemcontract.AddressListContractName], systemcontract.AddressListContractAddr, "rulesLen", 1)
	if err != nil {
		return 0, err
	}
	ln, ok := ret[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("unexpected output type, value: %v", ret[0])
	}
	return int(ln), nil
}

func (c *Congress) commonCallContract(header *types.Header, statedb *state.StateDB, contractABI abi.ABI, addr common.Address, method string, expectResultLen int, args ...interface{}) ([]interface{}, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		log.Error("Can't pack data ", "method", method, "err", err)
		return nil, err
	}

	msg := vmcaller.NewLegacyMessage(header.Coinbase, &addr, 0, new(big.Int), math.MaxUint64, new(big.Int), data, false)

	// Note: It's safe to use minimalChainContext for executing AddressListContract
	result, err := vmcaller.ExecuteMsg(msg, statedb, header, newMinimalChainContext(c), c.chainConfig)
	if err != nil {
		return nil, err
	}

	// unpack data
	ret, err := contractABI.Unpack(method, result)
	if err != nil {
		return nil, err
	}
	if len(ret) != expectResultLen {
		return nil, errors.New("invalid result length")
	}
	return ret, nil
}

// Since the state variables are as follow:
//    bool public initialized;
//    bool public enabled;
//    address public admin;
//    address public pendingAdmin;
//    mapping(address => bool) private devs;
//
// according to [Layout of State Variables in Storage](https://docs.soliditylang.org/en/v0.8.4/internals/layout_in_storage.html),
// and after optimizer enabled, the `initialized`, `enabled` and `admin` will be packed, and stores at slot 0,
// `pendingAdmin` stores at slot 1, and the position for `devs` is 2.
func isDeveloperVerificationEnabled(state consensus.StateReader) bool {
	compactValue := state.GetState(systemcontract.AddressListContractAddr, common.Hash{})
	// Layout of slot 0:
	// [0   -    9][10-29][  30   ][    31     ]
	// [zero bytes][admin][enabled][initialized]
	enabledByte := compactValue.Bytes()[common.HashLength-2]
	return enabledByte == 0x01
}

func calcSlotOfDevMappingKey(addr common.Address) common.Hash {
	p := make([]byte, common.HashLength)
	binary.BigEndian.PutUint16(p[common.HashLength-2:], uint16(systemcontract.DevMappingPosition))
	return crypto.Keccak256Hash(addr.Hash().Bytes(), p)
}

func lastBlacklistUpdatedNumber(state consensus.StateReader) uint64 {
	value := state.GetState(systemcontract.AddressListContractAddr, systemcontract.BlackLastUpdatedNumberPosition)
	return value.Big().Uint64()
}

func lastRulesUpdatedNumber(state consensus.StateReader) uint64 {
	value := state.GetState(systemcontract.AddressListContractAddr, systemcontract.RulesLastUpdatedNumberPosition)
	return value.Big().Uint64()
}
