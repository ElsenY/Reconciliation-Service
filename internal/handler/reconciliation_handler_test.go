package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/reconciliation-service/internal/dto"
	"github.com/reconciliation-service/internal/mocks"
	"go.uber.org/mock/gomock"
)

func formReq(body string) *http.Request {
	req := httptest.NewRequest("POST", "/api/reconcile", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func multipartReq(fields map[string]string, files map[string][]namedFile) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		w.WriteField(k, v)
	}
	for fieldName, nfs := range files {
		for _, nf := range nfs {
			part, _ := w.CreateFormFile(fieldName, nf.name)
			part.Write(nf.content)
		}
	}
	w.Close()

	req := httptest.NewRequest("POST", "/api/reconcile", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

type namedFile struct {
	name    string
	content []byte
}

// ---------------------------------------------------------------------------
// Validation tests (form-urlencoded) — no service call expected
// ---------------------------------------------------------------------------

func TestReconcile_MissingStartDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestReconcile_MissingEndDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestReconcile_InvalidStartDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=not-a-date&end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestReconcile_InvalidEndDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01&end_date=invalid"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestReconcile_MissingBothDates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq(""))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// No defaults configured — no service call expected
// ---------------------------------------------------------------------------

func TestReconcile_NoSystemDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "", []string{"/tmp/bank.csv"})
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01&end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 when no system default, got %d", resp.StatusCode)
	}
}

func TestReconcile_NoBankDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "/tmp/system.csv", nil)
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01&end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 when no bank default, got %d", resp.StatusCode)
	}
}

func TestReconcile_NoDefaultsAtAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01&end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 when no defaults, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Success / error with defaults — service IS called
// ---------------------------------------------------------------------------

func TestReconcile_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(nil, errors.New("load failed"))

	app := fiber.New()
	h := NewReconciliationHandler(svc, "/tmp/system.csv", []string{"/tmp/bank.csv"})
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01&end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("expected 500 on service error, got %d", resp.StatusCode)
	}
}

func TestReconcile_SuccessWithDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	summary := &dto.ReconciliationSummary{
		TotalMatchedTransactions:   5,
		TotalUnmatchedTransactions: 0,
	}
	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(summary, nil)

	app := fiber.New()
	h := NewReconciliationHandler(svc, "/tmp/system.csv", []string{"/tmp/bank.csv"})
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01&end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result dto.ReconciliationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
	if result.Data.TotalMatchedTransactions != 5 {
		t.Errorf("expected 5 matched, got %d", result.Data.TotalMatchedTransactions)
	}
}

func TestReconcile_SuccessWithMultipleDefaultBanks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	summary := &dto.ReconciliationSummary{TotalMatchedTransactions: 10}
	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(summary, nil)

	app := fiber.New()
	h := NewReconciliationHandler(svc, "/tmp/system.csv", []string{"/tmp/bca.csv", "/tmp/bni.csv", "/tmp/banka.csv"})
	h.RegisterRoutes(app)

	resp, err := app.Test(formReq("start_date=2024-01-01&end_date=2024-01-31"))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Multipart file upload tests — service IS called
// ---------------------------------------------------------------------------

func TestReconcile_UploadSystemTransactionsOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	summary := &dto.ReconciliationSummary{TotalMatchedTransactions: 1}
	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(summary, nil)

	app := fiber.New()
	h := NewReconciliationHandler(svc, "", []string{"/tmp/bank.csv"})
	h.RegisterRoutes(app)

	sysCSV := []byte("trxID,amount,type,transactionTime\nTRX001,100000,DEBIT,2024-01-10T10:00:00\n")
	req := multipartReq(
		map[string]string{"start_date": "2024-01-01", "end_date": "2024-01-31"},
		map[string][]namedFile{
			"system_transactions": {{name: "system.csv", content: sysCSV}},
		},
	)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestReconcile_UploadBankStatementsOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	summary := &dto.ReconciliationSummary{TotalMatchedTransactions: 1}
	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(summary, nil)

	app := fiber.New()
	h := NewReconciliationHandler(svc, "/tmp/system.csv", nil)
	h.RegisterRoutes(app)

	bankCSV := []byte("unique_identifier,amount,date\nBCA001,-100000,2024-01-10\n")
	req := multipartReq(
		map[string]string{"start_date": "2024-01-01", "end_date": "2024-01-31"},
		map[string][]namedFile{
			"bank_statements": {{name: "bank_bca.csv", content: bankCSV}},
		},
	)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestReconcile_UploadBothFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	summary := &dto.ReconciliationSummary{TotalMatchedTransactions: 2}
	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(summary, nil)

	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	sysCSV := []byte("trxID,amount,type,transactionTime\nTRX001,100000,DEBIT,2024-01-10T10:00:00\n")
	bankCSV := []byte("unique_identifier,amount,date\nBCA001,-100000,2024-01-10\n")
	req := multipartReq(
		map[string]string{"start_date": "2024-01-01", "end_date": "2024-01-31"},
		map[string][]namedFile{
			"system_transactions": {{name: "system.csv", content: sysCSV}},
			"bank_statements":     {{name: "bank_bca.csv", content: bankCSV}},
		},
	)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestReconcile_UploadMultipleBankFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	summary := &dto.ReconciliationSummary{TotalMatchedTransactions: 3}
	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(summary, nil)

	app := fiber.New()
	h := NewReconciliationHandler(svc, "/tmp/system.csv", nil)
	h.RegisterRoutes(app)

	bca := []byte("unique_identifier,amount,date\nBCA001,-100000,2024-01-10\n")
	bni := []byte("unique_identifier,amount,date\nBNI001,200000,2024-01-15\n")
	req := multipartReq(
		map[string]string{"start_date": "2024-01-01", "end_date": "2024-01-31"},
		map[string][]namedFile{
			"bank_statements": {
				{name: "bank_bca.csv", content: bca},
				{name: "bank_bni.csv", content: bni},
			},
		},
	)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestReconcile_UploadServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mocks.NewMockReconciliationService(ctrl)
	svc.EXPECT().Reconcile(gomock.Any()).Return(nil, fmt.Errorf("parse failed"))

	app := fiber.New()
	h := NewReconciliationHandler(svc, "", nil)
	h.RegisterRoutes(app)

	sysCSV := []byte("trxID,amount,type,transactionTime\nTRX001,100000,DEBIT,2024-01-10T10:00:00\n")
	bankCSV := []byte("unique_identifier,amount,date\nBCA001,-100000,2024-01-10\n")
	req := multipartReq(
		map[string]string{"start_date": "2024-01-01", "end_date": "2024-01-31"},
		map[string][]namedFile{
			"system_transactions": {{name: "system.csv", content: sysCSV}},
			"bank_statements":     {{name: "bank.csv", content: bankCSV}},
		},
	)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("expected 500 on service error with uploads, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// deriveBankSource
// ---------------------------------------------------------------------------

func TestDeriveBankSource(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"csv extension", "bank_bca.csv", "bank_bca"},
		{"with path", "/path/to/bank_bni.csv", "bank_bni"},
		{"no extension", "banka", "banka"},
		{"double extension", "bank.data.csv", "bank.data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveBankSource(tt.filename)
			if got != tt.want {
				t.Errorf("deriveBankSource(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
