package repository

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	fullRange = [2]time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
	}
)

// ---------------------------------------------------------------------------
// ParseSystemTransactions
// ---------------------------------------------------------------------------

func TestParseSystemTransactions_ValidCSV(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,100000,DEBIT,2024-01-10T10:00:00\nTRX002,200000,CREDIT,2024-01-10T11:00:00"
	repo := NewCSVTransactionRepository()
	got, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("ParseSystemTransactions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(got))
	}
	if got[0].TrxID != "TRX001" || got[0].Amount != 100000 || string(got[0].Type) != "DEBIT" {
		t.Errorf("first tx: got %+v", got[0])
	}
	if got[1].TrxID != "TRX002" || got[1].Amount != 200000 || string(got[1].Type) != "CREDIT" {
		t.Errorf("second tx: got %+v", got[1])
	}
}

func TestParseSystemTransactions_DateFilter(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,100000,DEBIT,2024-01-05T10:00:00\nTRX002,200000,CREDIT,2024-01-15T11:00:00\nTRX003,300000,DEBIT,2024-01-25T12:00:00"
	start := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 20, 23, 59, 59, 0, time.UTC)

	repo := NewCSVTransactionRepository()
	got, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), start, end)
	if err != nil {
		t.Fatalf("ParseSystemTransactions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 transaction in range, got %d", len(got))
	}
	if got[0].TrxID != "TRX002" {
		t.Errorf("expected TRX002, got %s", got[0].TrxID)
	}
}

func TestParseSystemTransactions_InvalidAmount(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,invalid,DEBIT,2024-01-10T10:00:00"
	repo := NewCSVTransactionRepository()
	_, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for invalid amount")
	}
}

func TestParseSystemTransactions_InvalidType(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,100000,INVALID,2024-01-10T10:00:00"
	repo := NewCSVTransactionRepository()
	_, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestParseSystemTransactions_InvalidDateTime(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,100000,DEBIT,not-a-date"
	repo := NewCSVTransactionRepository()
	_, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for invalid datetime")
	}
	if !strings.Contains(err.Error(), "invalid transaction time") {
		t.Errorf("error should mention invalid transaction time: %v", err)
	}
}

func TestParseSystemTransactions_TooFewColumns(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,100000"
	repo := NewCSVTransactionRepository()
	_, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for too few columns")
	}
}

func TestParseSystemTransactions_EmptyCSV(t *testing.T) {
	csv := "trxID,amount,type,transactionTime"
	repo := NewCSVTransactionRepository()
	got, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 transactions from header-only CSV, got %d", len(got))
	}
}

func TestParseSystemTransactions_LowercaseType(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,100000,debit,2024-01-10T10:00:00"
	repo := NewCSVTransactionRepository()
	got, err := repo.ParseSystemTransactions(bytes.NewReader([]byte(csv)), fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got[0].Type) != "DEBIT" {
		t.Errorf("expected type DEBIT, got %s", got[0].Type)
	}
}

// ---------------------------------------------------------------------------
// ParseBankStatements
// ---------------------------------------------------------------------------

func TestParseBankStatements_ValidCSV(t *testing.T) {
	csv := "unique_identifier,amount,date\nBCA001,-100000,2024-01-10\nBCA002,200000,2024-01-10"
	repo := NewCSVTransactionRepository()
	got, err := repo.ParseBankStatements(bytes.NewReader([]byte(csv)), "bank_bca", fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("ParseBankStatements: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(got))
	}
	if got[0].UniqueIdentifier != "BCA001" || got[0].Amount != 100000 || got[0].BankSource != "bank_bca" {
		t.Errorf("first stmt: got %+v", got[0])
	}
	if got[1].UniqueIdentifier != "BCA002" || got[1].Amount != 200000 {
		t.Errorf("second stmt: got %+v", got[1])
	}
}

func TestParseBankStatements_NegativeAmountStoredAsAbsolute(t *testing.T) {
	csv := "unique_identifier,amount,date\nBCA001,-50000,2024-01-10"
	repo := NewCSVTransactionRepository()
	got, err := repo.ParseBankStatements(bytes.NewReader([]byte(csv)), "bank_bca", fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("ParseBankStatements: %v", err)
	}
	if got[0].Amount != 50000 {
		t.Errorf("expected absolute amount 50000, got %f", got[0].Amount)
	}
}

func TestParseBankStatements_DateFilter(t *testing.T) {
	csv := "unique_identifier,amount,date\nBCA001,100000,2024-01-05\nBCA002,200000,2024-01-15\nBCA003,300000,2024-01-25"
	start := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC)

	repo := NewCSVTransactionRepository()
	got, err := repo.ParseBankStatements(bytes.NewReader([]byte(csv)), "bank_bca", start, end)
	if err != nil {
		t.Fatalf("ParseBankStatements: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 statement in range, got %d", len(got))
	}
	if got[0].UniqueIdentifier != "BCA002" {
		t.Errorf("expected BCA002, got %s", got[0].UniqueIdentifier)
	}
}

func TestParseBankStatements_InvalidAmount(t *testing.T) {
	csv := "unique_identifier,amount,date\nBCA001,xyz,2024-01-10"
	repo := NewCSVTransactionRepository()
	_, err := repo.ParseBankStatements(bytes.NewReader([]byte(csv)), "bank_bca", fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for invalid bank amount")
	}
	if !strings.Contains(err.Error(), "invalid amount") {
		t.Errorf("error should mention invalid amount: %v", err)
	}
}

func TestParseBankStatements_InvalidDate(t *testing.T) {
	csv := "unique_identifier,amount,date\nBCA001,100000,not-a-date"
	repo := NewCSVTransactionRepository()
	_, err := repo.ParseBankStatements(bytes.NewReader([]byte(csv)), "bank_bca", fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for invalid bank date")
	}
	if !strings.Contains(err.Error(), "invalid date") {
		t.Errorf("error should mention invalid date: %v", err)
	}
}

func TestParseBankStatements_TooFewColumns(t *testing.T) {
	csv := "unique_identifier,amount,date\nBCA001,100000"
	repo := NewCSVTransactionRepository()
	_, err := repo.ParseBankStatements(bytes.NewReader([]byte(csv)), "bank_bca", fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for too few columns")
	}
}

func TestParseBankStatements_EmptyCSV(t *testing.T) {
	csv := "unique_identifier,amount,date"
	repo := NewCSVTransactionRepository()
	got, err := repo.ParseBankStatements(bytes.NewReader([]byte(csv)), "bank_bca", fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 statements from header-only CSV, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// GetSystemTransactions (file-based)
// ---------------------------------------------------------------------------

func TestGetSystemTransactions_FileNotFound(t *testing.T) {
	repo := NewCSVTransactionRepository()
	_, err := repo.GetSystemTransactions("/nonexistent/file.csv", fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGetSystemTransactions_FromTempFile(t *testing.T) {
	csv := "trxID,amount,type,transactionTime\nTRX001,100000,DEBIT,2024-01-10T10:00:00\nTRX002,200000,CREDIT,2024-01-15T11:00:00\n"
	f, err := os.CreateTemp(t.TempDir(), "sys-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(csv)
	f.Close()

	repo := NewCSVTransactionRepository()
	got, err := repo.GetSystemTransactions(f.Name(), fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("GetSystemTransactions: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// GetBankStatements (file-based)
// ---------------------------------------------------------------------------

func TestGetBankStatements_FileNotFound(t *testing.T) {
	repo := NewCSVTransactionRepository()
	_, err := repo.GetBankStatements("/nonexistent/file.csv", "test_bank", fullRange[0], fullRange[1])
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGetBankStatements_FromTempFile(t *testing.T) {
	csv := "unique_identifier,amount,date\nBCA001,-100000,2024-01-10\nBCA002,200000,2024-01-15\n"
	f, err := os.CreateTemp(t.TempDir(), "bank-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(csv)
	f.Close()

	repo := NewCSVTransactionRepository()
	got, err := repo.GetBankStatements(f.Name(), filepath.Base(f.Name()), fullRange[0], fullRange[1])
	if err != nil {
		t.Fatalf("GetBankStatements: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(got))
	}
	if got[0].Amount != 100000 {
		t.Errorf("expected abs(100000), got %f", got[0].Amount)
	}
}
