package repository

import (
	"io"
	"time"

	"github.com/reconciliation-service/internal/model"
)

type TransactionRepository interface {
	GetSystemTransactions(filePath string, startDate, endDate time.Time) ([]model.Transaction, error)
	GetBankStatements(filePath string, bankSource string, startDate, endDate time.Time) ([]model.BankStatement, error)

	ParseSystemTransactions(r io.Reader, startDate, endDate time.Time) ([]model.Transaction, error)
	ParseBankStatements(r io.Reader, bankSource string, startDate, endDate time.Time) ([]model.BankStatement, error)
}
