package service

import (
	"fmt"
	"math"
	"time"

	"github.com/reconciliation-service/internal/dto"
	"github.com/reconciliation-service/internal/model"
	"github.com/reconciliation-service/internal/repository"
)

type reconciliationService struct {
	repo repository.TransactionRepository
}

type ReconciliationService interface {
	Reconcile(req dto.ReconciliationRequest) (*dto.ReconciliationSummary, error)
}

func NewReconciliationService(repo repository.TransactionRepository) ReconciliationService {
	return &reconciliationService{repo: repo}
}

func (s *reconciliationService) Reconcile(req dto.ReconciliationRequest) (*dto.ReconciliationSummary, error) {
	// read system transactions
	var systemTrx []model.Transaction
	var err error
	if req.SystemTransactionReader != nil {
		systemTrx, err = s.repo.ParseSystemTransactions(req.SystemTransactionReader, req.StartDate, req.EndDate)
	} else {
		systemTrx, err = s.repo.GetSystemTransactions(req.SystemTransactionFilePath, req.StartDate, req.EndDate)
	}

	if err != nil {
		return nil, fmt.Errorf("loading system transactions: %w", err)
	}

	// read bank statements inputs
	bankStmts := []model.BankStatement{}
	for _, input := range req.BankInputs {
		var bankStmt []model.BankStatement
		if input.Reader != nil {
			bankStmt, err = s.repo.ParseBankStatements(input.Reader, input.BankSource, req.StartDate, req.EndDate)
		} else {
			bankStmt, err = s.repo.GetBankStatements(input.FilePath, input.BankSource, req.StartDate, req.EndDate)
		}

		if err != nil {
			return nil, fmt.Errorf("loading bank statements: %w", err)
		}

		bankStmts = append(bankStmts, bankStmt...)
	}

	return reconcile(systemTrx, bankStmts), nil
}

// struct key for map so that we can use it to mark transactions in X date
type dateKey struct {
	year  int
	month time.Month
	day   int
}

// convert go time.Time to dateKey
func toDateKey(t time.Time) dateKey {
	return dateKey{year: t.Year(), month: t.Month(), day: t.Day()}
}

func buildBankIndex(bankStmts []model.BankStatement) map[dateKey]*model.DateBucket {
	idx := make(map[dateKey]*model.DateBucket)
	for _, stmt := range bankStmts {
		dk := toDateKey(stmt.Date)
		bucket, ok := idx[dk]
		if !ok {
			bucket = &model.DateBucket{
				ByAmount: make(map[int64][]int),
			}
			idx[dk] = bucket
		}
		entryIdx := len(bucket.Entries)
		bucket.Entries = append(bucket.Entries, stmt)
		bucket.Matched = append(bucket.Matched, false)

		// rounding to 3 decimal places
		amtKey := int64(math.Round(stmt.Amount * 1000))
		bucket.ByAmount[amtKey] = append(bucket.ByAmount[amtKey], entryIdx)
	}
	return idx
}

func reconcile(systemTxns []model.Transaction, bankStmts []model.BankStatement) *dto.ReconciliationSummary {
	summary := &dto.ReconciliationSummary{
		MissingInSystem: make(map[string][]dto.UnmatchedTransaction),
	}
	summary.TotalSystemTransactionsProcessed = len(systemTxns)
	summary.TotalBankStatementsProcessed = len(bankStmts)
	summary.TotalStatementsProcessed = len(systemTxns) + len(bankStmts)

	// build bank index for O(1) lookups
	bankIdx := buildBankIndex(bankStmts)

	// track which system transaction that have been or not yet matched
	systemMatched := make([]bool, len(systemTxns))

	// step 1: match system transactions that have an exact date+amount match.
	for i, sysTx := range systemTxns {
		dk := toDateKey(sysTx.TransactionTime)
		bucket, ok := bankIdx[dk]
		if !ok {
			continue
		}
		entryIdx := bucket.FindExactMatch(sysTx.Amount)
		if entryIdx < 0 {
			continue
		}
		systemMatched[i] = true
		bucket.Matched[entryIdx] = true
		summary.TotalMatchedTransactions++
	}

	// step 2: for remaining unmatched system transactions, find the closest
	// amount on the same date (reports as discrepancy).
	for i, sysTx := range systemTxns {
		if systemMatched[i] {
			continue
		}
		dk := toDateKey(sysTx.TransactionTime)
		bucket, ok := bankIdx[dk]
		if !ok {
			continue
		}
		entryIdx := bucket.FindClosestMatch(sysTx.Amount)
		if entryIdx < 0 {
			continue
		}
		entry := bucket.Entries[entryIdx]
		systemMatched[i] = true
		bucket.Matched[entryIdx] = true
		summary.TotalMatchedTransactions++

		diff := math.Abs(sysTx.Amount - entry.Amount)
		if diff > 0.001 {
			summary.TotalDiscrepancyAmount += diff
			summary.Discrepancies = append(summary.Discrepancies, dto.DiscrepantTransaction{
				SystemTrxID:    sysTx.TrxID,
				BankIdentifier: entry.UniqueIdentifier,
				SystemAmount:   sysTx.Amount,
				BankAmount:     entry.Amount,
				Difference:     diff,
				BankSource:     entry.BankSource,
			})
		}
	}

	// step 3: track which system transaction that have not been matched
	for i, matched := range systemMatched {
		if !matched {
			tx := systemTxns[i]
			summary.MissingInBank = append(summary.MissingInBank, dto.UnmatchedTransaction{
				ID:     tx.TrxID,
				Amount: tx.Amount,
				Date:   tx.TransactionTime,
				Type:   string(tx.Type),
			})
		}
	}

	// step 4: track which bank statement that have not been matched
	for _, bucket := range bankIdx {
		for i, matched := range bucket.Matched {
			if !matched {
				stmt := bucket.Entries[i]
				summary.MissingInSystem[stmt.BankSource] = append(
					summary.MissingInSystem[stmt.BankSource],
					dto.UnmatchedTransaction{
						ID:     stmt.UniqueIdentifier,
						Amount: stmt.Amount,
						Date:   stmt.Date,
					},
				)
			}
		}
	}

	// calculate total unmatched transactions
	summary.TotalUnmatchedTransactions = len(summary.MissingInBank)
	for _, unmatched := range summary.MissingInSystem {
		summary.TotalUnmatchedTransactions += len(unmatched)
	}

	return summary
}
