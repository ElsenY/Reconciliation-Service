package model

import (
	"math"
	"time"
)

type TransactionType string

type Transaction struct {
	TrxID           string
	Amount          float64
	Type            TransactionType
	TransactionTime time.Time
}

type BankStatement struct {
	UniqueIdentifier string
	Amount           float64
	Date             time.Time
	BankSource       string
}

type DateBucket struct {
	ByAmount map[int64][]int
	Entries  []BankStatement
	Matched  []bool
}

// find the exact match bank statement with system transaction
func (b *DateBucket) FindExactMatch(amount float64) int {
	// rounding to 3 decimal places
	key := int64(math.Round(amount * 1000))
	for _, entryIdx := range b.ByAmount[key] {
		if !b.Matched[entryIdx] {
			return entryIdx
		}
	}
	return -1
}

// find the closest match bank statement with system transaction (for discrepancy)
func (b *DateBucket) FindClosestMatch(amount float64) int {
	bestIdx := -1
	bestDiff := math.MaxFloat64
	for i, entry := range b.Entries {
		if b.Matched[i] {
			continue
		}
		diff := math.Abs(amount - entry.Amount)
		if diff < bestDiff {
			bestDiff = diff
			bestIdx = i
		}
	}
	return bestIdx
}
