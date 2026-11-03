package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const GenesisBlockHash = "0000000000000000000000000000000000000000000000000000000000000000"

// ComputePayloadHash returns the SHA-256 hash of a payload string
func ComputePayloadHash(payload string) string {
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

// ComputeAuditBlockHash computes the SHA-256 hash of a block linking to prevBlockHash
func ComputeAuditBlockHash(seqNum int64, prevBlockHash, action, actorID, payloadHash, timestampStr string) string {
	raw := fmt.Sprintf("%d:%s:%s:%s:%s:%s", seqNum, prevBlockHash, action, actorID, payloadHash, timestampStr)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ChainBlockVerificationItem represents a record in the chain to be verified
type ChainBlockVerificationItem struct {
	SequenceNumber int64
	PrevBlockHash  string
	BlockHash      string
	Action         string
	ActorID        string
	PayloadHash    string
	Timestamp      time.Time
}

// VerifyAuditChain traverses an array of chain blocks and validates sequential mathematical integrity
func VerifyAuditChain(blocks []ChainBlockVerificationItem) (bool, int, error) {
	if len(blocks) == 0 {
		return true, 0, nil
	}

	for i, b := range blocks {
		expectedPrev := GenesisBlockHash
		if i > 0 {
			expectedPrev = blocks[i-1].BlockHash
			if b.SequenceNumber != blocks[i-1].SequenceNumber+1 {
				return false, i, fmt.Errorf("sequence gap at index %d: expected %d, got %d",
					i, blocks[i-1].SequenceNumber+1, b.SequenceNumber)
			}
		}

		if b.PrevBlockHash != expectedPrev {
			return false, i, fmt.Errorf("broken chain link at block seq %d: prev hash mismatch", b.SequenceNumber)
		}

		tsStr := b.Timestamp.UTC().Format(time.RFC3339)
		computedHash := ComputeAuditBlockHash(b.SequenceNumber, b.PrevBlockHash, b.Action, b.ActorID, b.PayloadHash, tsStr)
		if computedHash != b.BlockHash {
			return false, i, fmt.Errorf("tamper detected at block seq %d: expected hash %s, computed %s",
				b.SequenceNumber, b.BlockHash, computedHash)
		}
	}

	return true, len(blocks), nil
}

