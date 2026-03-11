package handler

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/reconciliation-service/internal/dto"
	"github.com/reconciliation-service/internal/service"
)

type ReconciliationHandler struct {
	service               service.ReconciliationService
	defaultSystemFilePath string
	defaultBankFilePaths  []string
}

func NewReconciliationHandler(svc service.ReconciliationService, defaultSystemFilePath string, defaultBankFilePaths []string) *ReconciliationHandler {
	return &ReconciliationHandler{
		service:               svc,
		defaultSystemFilePath: defaultSystemFilePath,
		defaultBankFilePaths:  defaultBankFilePaths,
	}
}

func (h *ReconciliationHandler) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api")
	api.Post("/reconcile", h.Reconcile)
}

func (h *ReconciliationHandler) Reconcile(c *fiber.Ctx) error {
	startDateStr := c.FormValue("start_date")
	endDateStr := c.FormValue("end_date")

	if startDateStr == "" || endDateStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "start_date and end_date are required (format: YYYY-MM-DD)",
		})
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid start_date: %v", err),
		})
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid end_date: %v", err),
		})
	}
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	req := dto.ReconciliationRequest{
		StartDate: startDate,
		EndDate:   endDate,
	}

	// read system transactions inputs
	systemFile, err := c.FormFile("system_transactions")
	if err == nil && systemFile != nil {
		reader, err := fileHeaderToReader(systemFile)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to read system_transactions file: %v", err),
			})
		}
		req.SystemTransactionReader = reader
	} else {
		// use default system transactions file path if file not provided
		if h.defaultSystemFilePath == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "system_transactions file is required (no default configured)",
			})
		}
		req.SystemTransactionFilePath = h.defaultSystemFilePath
	}

	form, err := c.MultipartForm()
	if err == nil && form != nil {
		// read bank statements inputs
		if bankFiles, ok := form.File["bank_statements"]; ok && len(bankFiles) > 0 {
			for _, fh := range bankFiles {
				reader, err := fileHeaderToReader(fh)
				if err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": fmt.Sprintf("failed to read bank statement file %s: %v", fh.Filename, err),
					})
				}
				req.BankInputs = append(req.BankInputs, dto.BankFileInput{
					BankSource: deriveBankSource(fh.Filename),
					Reader:     reader,
				})
			}
		}
	}

	// use default bank statements file paths if file not provided
	if len(req.BankInputs) == 0 {
		if len(h.defaultBankFilePaths) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "bank_statements files are required (no defaults configured)",
			})
		}
		for _, fp := range h.defaultBankFilePaths {
			req.BankInputs = append(req.BankInputs, dto.BankFileInput{
				FilePath:   fp,
				BankSource: deriveBankSource(fp),
			})
		}
	}

	// reconcile
	summary, err := h.service.Reconcile(req)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("reconciliation failed: %v", err),
		})
	}

	return c.JSON(dto.ReconciliationResponse{
		Code:    200,
		Status:  "success",
		Message: "Reconciliation data fetched",
		Data:    summary,
	})
}

func fileHeaderToReader(fh *multipart.FileHeader) (*bytes.Reader, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

func deriveBankSource(filename string) string {
	base := filepath.Base(filename)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return name
}
