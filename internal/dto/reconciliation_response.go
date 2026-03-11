package dto

import "time"

type UnmatchedTransaction struct {
	ID     string    `json:"id"`
	Amount float64   `json:"amount"`
	Date   time.Time `json:"date"`
	Type   string    `json:"type,omitempty"`
}

type DiscrepantTransaction struct {
	SystemTrxID    string  `json:"system_trx_id"`
	BankIdentifier string  `json:"bank_identifier"`
	SystemAmount   float64 `json:"system_amount"`
	BankAmount     float64 `json:"bank_amount"`
	Difference     float64 `json:"difference"`
	BankSource     string  `json:"bank_source"`
}

type ReconciliationSummary struct {
	TotalStatementsProcessed         int `json:"total_statements_processed"`
	TotalBankStatementsProcessed     int `json:"total_bank_statements_processed"`
	TotalSystemTransactionsProcessed int `json:"total_system_transactions_processed"`
	TotalMatchedTransactions         int `json:"total_matched_transactions"`
	TotalUnmatchedTransactions       int `json:"total_unmatched_transactions"`

	MissingInBank   []UnmatchedTransaction            `json:"missing_in_bank"`
	MissingInSystem map[string][]UnmatchedTransaction `json:"missing_in_system"`

	TotalDiscrepancyAmount float64                 `json:"total_discrepancy_amount"`
	Discrepancies          []DiscrepantTransaction `json:"discrepancies"`
}

type ReconciliationResponse struct {
	Code    int                    `json:"code"`
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    *ReconciliationSummary `json:"data"`
}
