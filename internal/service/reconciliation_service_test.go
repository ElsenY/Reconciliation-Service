package service

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/reconciliation-service/internal/dto"
	"github.com/reconciliation-service/internal/mocks"
	"github.com/reconciliation-service/internal/model"
	"go.uber.org/mock/gomock"
)

func TestReconcile_ExactMatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	date := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	endDate := date.Add(24 * time.Hour)

	sys := []model.Transaction{
		{TrxID: "TRX001", Amount: 100000, Type: "DEBIT", TransactionTime: date},
		{TrxID: "TRX002", Amount: 200000, Type: "CREDIT", TransactionTime: date},
	}
	bank := []model.BankStatement{
		{UniqueIdentifier: "BCA001", Amount: 100000, Date: date, BankSource: "bank_bca"},
		{UniqueIdentifier: "BCA002", Amount: 200000, Date: date, BankSource: "bank_bca"},
	}

	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), date, endDate).Return(sys, nil)
	repo.EXPECT().ParseBankStatements(gomock.Any(), "bank_bca", date, endDate).Return(bank, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               date,
		EndDate:                 endDate,
		SystemTransactionReader: bytes.NewReader(nil),
		BankInputs: []dto.BankFileInput{
			{BankSource: "bank_bca", Reader: bytes.NewReader(nil)},
		},
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if summary.TotalMatchedTransactions != 2 {
		t.Errorf("expected 2 matched, got %d", summary.TotalMatchedTransactions)
	}
	if summary.TotalUnmatchedTransactions != 0 {
		t.Errorf("expected 0 unmatched, got %d", summary.TotalUnmatchedTransactions)
	}
	if len(summary.Discrepancies) != 0 {
		t.Errorf("expected 0 discrepancies, got %d", len(summary.Discrepancies))
	}
}

func TestReconcile_Discrepancy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	date := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	endDate := date.Add(24 * time.Hour)

	sys := []model.Transaction{
		{TrxID: "TRX001", Amount: 100000, Type: "DEBIT", TransactionTime: date},
	}
	bank := []model.BankStatement{
		{UniqueIdentifier: "BCA001", Amount: 105000, Date: date, BankSource: "bank_bca"},
	}

	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), date, endDate).Return(sys, nil)
	repo.EXPECT().ParseBankStatements(gomock.Any(), "bank_bca", date, endDate).Return(bank, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               date,
		EndDate:                 endDate,
		SystemTransactionReader: bytes.NewReader(nil),
		BankInputs: []dto.BankFileInput{
			{BankSource: "bank_bca", Reader: bytes.NewReader(nil)},
		},
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if summary.TotalMatchedTransactions != 1 {
		t.Errorf("expected 1 matched, got %d", summary.TotalMatchedTransactions)
	}
	if len(summary.Discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d", len(summary.Discrepancies))
	}
	d := summary.Discrepancies[0]
	if d.SystemTrxID != "TRX001" || d.BankIdentifier != "BCA001" {
		t.Errorf("discrepancy ids: got %s / %s", d.SystemTrxID, d.BankIdentifier)
	}
	if d.Difference < 4999 || d.Difference > 5001 {
		t.Errorf("expected difference ~5000, got %f", d.Difference)
	}
}

func TestReconcile_MissingInBank(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	date := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	endDate := date.Add(24 * time.Hour)

	sys := []model.Transaction{
		{TrxID: "TRX001", Amount: 100000, Type: "DEBIT", TransactionTime: date},
		{TrxID: "TRX002", Amount: 200000, Type: "CREDIT", TransactionTime: date},
	}
	bank := []model.BankStatement{
		{UniqueIdentifier: "BCA001", Amount: 100000, Date: date, BankSource: "bank_bca"},
	}

	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), date, endDate).Return(sys, nil)
	repo.EXPECT().ParseBankStatements(gomock.Any(), "bank_bca", date, endDate).Return(bank, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               date,
		EndDate:                 endDate,
		SystemTransactionReader: bytes.NewReader(nil),
		BankInputs: []dto.BankFileInput{
			{BankSource: "bank_bca", Reader: bytes.NewReader(nil)},
		},
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(summary.MissingInBank) != 1 {
		t.Fatalf("expected 1 missing in bank, got %d", len(summary.MissingInBank))
	}
	if summary.MissingInBank[0].ID != "TRX002" {
		t.Errorf("expected TRX002 missing in bank, got %s", summary.MissingInBank[0].ID)
	}
}

func TestReconcile_MissingInSystem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	date := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	endDate := date.Add(24 * time.Hour)

	sys := []model.Transaction{
		{TrxID: "TRX001", Amount: 100000, Type: "DEBIT", TransactionTime: date},
	}
	bank := []model.BankStatement{
		{UniqueIdentifier: "BCA001", Amount: 100000, Date: date, BankSource: "bank_bca"},
		{UniqueIdentifier: "BCA002", Amount: 300000, Date: date, BankSource: "bank_bca"},
	}

	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), date, endDate).Return(sys, nil)
	repo.EXPECT().ParseBankStatements(gomock.Any(), "bank_bca", date, endDate).Return(bank, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               date,
		EndDate:                 endDate,
		SystemTransactionReader: bytes.NewReader(nil),
		BankInputs: []dto.BankFileInput{
			{BankSource: "bank_bca", Reader: bytes.NewReader(nil)},
		},
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	missing := summary.MissingInSystem["bank_bca"]
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing in system for bank_bca, got %d", len(missing))
	}
	if missing[0].ID != "BCA002" {
		t.Errorf("expected BCA002 missing in system, got %s", missing[0].ID)
	}
}

func TestReconcile_SystemLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().GetSystemTransactions("/nonexistent.csv", now, now).Return(nil, errors.New("file not found"))

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:                 now,
		EndDate:                   now,
		SystemTransactionFilePath: "/nonexistent.csv",
	}

	_, err := svc.Reconcile(req)
	if err == nil {
		t.Fatal("expected error when system load fails")
	}
}

func TestReconcile_BankLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), now, now).Return([]model.Transaction{}, nil)
	repo.EXPECT().ParseBankStatements(gomock.Any(), "bank_bca", now, now).Return(nil, errors.New("bank parse error"))

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               now,
		EndDate:                 now,
		SystemTransactionReader: bytes.NewReader(nil),
		BankInputs: []dto.BankFileInput{
			{BankSource: "bank_bca", Reader: bytes.NewReader(nil)},
		},
	}

	_, err := svc.Reconcile(req)
	if err == nil {
		t.Fatal("expected error when bank load fails")
	}
}

func TestReconcile_BankFilePathFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	date := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	endDate := date.Add(24 * time.Hour)

	sys := []model.Transaction{
		{TrxID: "TRX001", Amount: 100000, Type: "DEBIT", TransactionTime: date},
	}
	bank := []model.BankStatement{
		{UniqueIdentifier: "BCA001", Amount: 100000, Date: date, BankSource: "bank_bca"},
	}

	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().GetSystemTransactions("/tmp/system.csv", date, endDate).Return(sys, nil)
	repo.EXPECT().GetBankStatements("/tmp/bank_bca.csv", "bank_bca", date, endDate).Return(bank, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:                 date,
		EndDate:                   endDate,
		SystemTransactionFilePath: "/tmp/system.csv",
		BankInputs: []dto.BankFileInput{
			{BankSource: "bank_bca", FilePath: "/tmp/bank_bca.csv"},
		},
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if summary.TotalMatchedTransactions != 1 {
		t.Errorf("expected 1 match, got %d", summary.TotalMatchedTransactions)
	}
}

func TestReconcile_MultipleDatesWithDiscrepancies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d1 := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 3, 12, 15, 0, 0, 0, time.UTC)
	endDate := d3.Add(24 * time.Hour)

	sys := []model.Transaction{
		{TrxID: "TRX001", Amount: 100000, Type: "DEBIT", TransactionTime: d1},
		{TrxID: "TRX002", Amount: 200000, Type: "CREDIT", TransactionTime: d1},
		{TrxID: "TRX003", Amount: 300000, Type: "DEBIT", TransactionTime: d2},
		{TrxID: "TRX004", Amount: 400000, Type: "CREDIT", TransactionTime: d3},
		{TrxID: "TRX005", Amount: 500000, Type: "DEBIT", TransactionTime: d3},
	}
	bank := []model.BankStatement{
		{UniqueIdentifier: "BCA001", Amount: 100000, Date: d1, BankSource: "bank_bca"},
		{UniqueIdentifier: "BNI001", Amount: 200000, Date: d1, BankSource: "bank_bni"},
		{UniqueIdentifier: "BCA002", Amount: 305000, Date: d2, BankSource: "bank_bca"},
		{UniqueIdentifier: "BNI002", Amount: 400000, Date: d3, BankSource: "bank_bni"},
		{UniqueIdentifier: "BNI003", Amount: 510000, Date: d3, BankSource: "bank_bni"},
	}

	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), d1, endDate).Return(sys, nil)
	repo.EXPECT().ParseBankStatements(gomock.Any(), "mixed", d1, endDate).Return(bank, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               d1,
		EndDate:                 endDate,
		SystemTransactionReader: bytes.NewReader(nil),
		BankInputs: []dto.BankFileInput{
			{BankSource: "mixed", Reader: bytes.NewReader(nil)},
		},
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if summary.TotalMatchedTransactions != 5 {
		t.Errorf("expected 5 matched, got %d", summary.TotalMatchedTransactions)
	}
	if summary.TotalSystemTransactionsProcessed != 5 {
		t.Errorf("expected 5 system processed, got %d", summary.TotalSystemTransactionsProcessed)
	}
	if summary.TotalBankStatementsProcessed != 5 {
		t.Errorf("expected 5 bank processed, got %d", summary.TotalBankStatementsProcessed)
	}
	if len(summary.Discrepancies) != 2 {
		t.Errorf("expected 2 discrepancies, got %d", len(summary.Discrepancies))
	}
}

func TestReconcile_EmptyInputs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), now, now).Return(nil, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               now,
		EndDate:                 now,
		SystemTransactionReader: bytes.NewReader(nil),
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if summary.TotalMatchedTransactions != 0 {
		t.Errorf("expected 0 matched, got %d", summary.TotalMatchedTransactions)
	}
	if summary.TotalUnmatchedTransactions != 0 {
		t.Errorf("expected 0 unmatched, got %d", summary.TotalUnmatchedTransactions)
	}
}

func TestReconcile_ExactMatchPreferredOverClosest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	date := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	endDate := date.Add(24 * time.Hour)

	sys := []model.Transaction{
		{TrxID: "TRX001", Amount: 100000, Type: "DEBIT", TransactionTime: date},
		{TrxID: "TRX002", Amount: 105000, Type: "DEBIT", TransactionTime: date},
	}
	bank := []model.BankStatement{
		{UniqueIdentifier: "BCA001", Amount: 200000, Date: date, BankSource: "bank_bca"},
		{UniqueIdentifier: "BNI001", Amount: 100000, Date: date, BankSource: "bank_bni"},
	}

	repo := mocks.NewMockTransactionRepository(ctrl)
	repo.EXPECT().ParseSystemTransactions(gomock.Any(), date, endDate).Return(sys, nil)
	repo.EXPECT().ParseBankStatements(gomock.Any(), "bank_bca", date, endDate).Return(bank, nil)

	svc := NewReconciliationService(repo)
	req := dto.ReconciliationRequest{
		StartDate:               date,
		EndDate:                 endDate,
		SystemTransactionReader: bytes.NewReader(nil),
		BankInputs: []dto.BankFileInput{
			{BankSource: "bank_bca", Reader: bytes.NewReader(nil)},
		},
	}

	summary, err := svc.Reconcile(req)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if summary.TotalMatchedTransactions != 2 {
		t.Errorf("expected 2 matched, got %d", summary.TotalMatchedTransactions)
	}
	if len(summary.Discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d", len(summary.Discrepancies))
	}
	d := summary.Discrepancies[0]
	if d.SystemTrxID != "TRX002" {
		t.Errorf("expected TRX002 to have discrepancy, got %s", d.SystemTrxID)
	}
}
