package dto

import (
	"io"
	"time"
)

type BankFileInput struct {
	FilePath   string
	BankSource string
	Reader     io.Reader
}

type ReconciliationRequest struct {
	SystemTransactionFilePath string
	SystemTransactionReader   io.Reader

	BankInputs []BankFileInput

	StartDate time.Time
	EndDate   time.Time
}
