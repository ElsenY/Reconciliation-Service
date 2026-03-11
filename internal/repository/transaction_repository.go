package repository

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/reconciliation-service/internal/helper/constant"
	"github.com/reconciliation-service/internal/helper/parser"
	"github.com/reconciliation-service/internal/model"
)

type TransactionRepository interface {
	GetSystemTransactions(filePath string, startDate, endDate time.Time) ([]model.Transaction, error)
	GetBankStatements(filePath string, bankSource string, startDate, endDate time.Time) ([]model.BankStatement, error)

	ParseSystemTransactions(r io.Reader, startDate, endDate time.Time) ([]model.Transaction, error)
	ParseBankStatements(r io.Reader, bankSource string, startDate, endDate time.Time) ([]model.BankStatement, error)
}
type csvTransactionRepository struct{}

func NewCSVTransactionRepository() TransactionRepository {
	return &csvTransactionRepository{}
}

func (r *csvTransactionRepository) GetSystemTransactions(filePath string, startDate, endDate time.Time) ([]model.Transaction, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading system transactions from %s: %w", filePath, err)
	}
	defer f.Close()
	return r.ParseSystemTransactions(f, startDate, endDate)
}

func (r *csvTransactionRepository) ParseSystemTransactions(reader io.Reader, startDate, endDate time.Time) ([]model.Transaction, error) {
	records, err := parser.ReadCSVFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("parsing system transactions: %w", err)
	}

	var transactions []model.Transaction
	for i, record := range records {
		if len(record) < 4 {
			return nil, fmt.Errorf("row %d: expected at least 4 columns, got %d", i+2, len(record))
		}

		amount, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid amount %q: %w", i+2, record[1], err)
		}

		txType := model.TransactionType(strings.TrimSpace(strings.ToUpper(record[2])))
		if txType != constant.TransactionTypeDebit && txType != constant.TransactionTypeCredit {
			return nil, fmt.Errorf("row %d: invalid transaction type %q", i+2, record[2])
		}

		txTime, err := parser.ParseDateTime(strings.TrimSpace(record[3]))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid transaction time %q: %w", i+2, record[3], err)
		}

		if txTime.Before(startDate) || txTime.After(endDate) {
			continue
		}

		transactions = append(transactions, model.Transaction{
			TrxID:           strings.TrimSpace(record[0]),
			Amount:          amount,
			Type:            txType,
			TransactionTime: txTime,
		})
	}

	return transactions, nil
}

func (r *csvTransactionRepository) GetBankStatements(filePath string, bankSource string, startDate, endDate time.Time) ([]model.BankStatement, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading bank statements from %s: %w", filePath, err)
	}
	defer f.Close()
	return r.ParseBankStatements(f, bankSource, startDate, endDate)
}

func (r *csvTransactionRepository) ParseBankStatements(reader io.Reader, bankSource string, startDate, endDate time.Time) ([]model.BankStatement, error) {
	records, err := parser.ReadCSVFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("parsing bank statements: %w", err)
	}

	var statements []model.BankStatement
	for i, record := range records {
		if len(record) < 3 {
			return nil, fmt.Errorf("row %d: expected at least 3 columns, got %d", i+2, len(record))
		}

		amount, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid amount %q: %w", i+2, record[1], err)
		}

		txDate, err := parser.ParseDate(strings.TrimSpace(record[2]))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid date %q: %w", i+2, record[2], err)
		}

		if txDate.Before(startDate) || txDate.After(endDate) {
			continue
		}

		statements = append(statements, model.BankStatement{
			UniqueIdentifier: strings.TrimSpace(record[0]),
			Amount:           math.Abs(amount),
			Date:             txDate,
			BankSource:       bankSource,
		})
	}

	return statements, nil
}
