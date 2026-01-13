package api

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"

	"github.com/banking/transaction-service/internal/domain"
	"github.com/banking/transaction-service/internal/service"
)

type Handler struct {
	svc *service.TransactionService
}

func NewHandler(svc *service.TransactionService) *Handler {
	return &Handler{svc: svc}
}

type TransferRequest struct {
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	Amount        string `json:"amount"` // Decimal as string
	Currency      string `json:"currency"`
	Memo          string `json:"memo"`
}

func (h *Handler) InitiateTransfer(c echo.Context) error {
	var req TransferRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing user id"})
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}

	fromID, err := uuid.Parse(req.FromAccountID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid from_account_id"})
	}

	toID, err := uuid.Parse(req.ToAccountID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid to_account_id"})
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid amount"})
	}

	createReq := service.CreateTransferRequest{
		UserID:           uid,
		FromAccountID:    fromID,
		ToAccountID:      toID,
		Amount:           amount,
		Currency:         req.Currency,
		TransferType:     domain.TransferTypeInternal, // Default for now
		InitiationMethod: domain.InitiationMethodAPI,
		Memo:             req.Memo,
		Metadata: map[string]string{
			"source_ip":  c.RealIP(),
			"user_agent": c.Request().UserAgent(),
		},
	}

	txn, err := h.svc.InitiateTransfer(c.Request().Context(), createReq)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, txn)
}

func (h *Handler) GetTransaction(c echo.Context) error {
	id := c.Param("id")
	txnID, err := uuid.Parse(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid transaction id"})
	}

	txn, err := h.svc.GetTransaction(c.Request().Context(), txnID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "transaction not found"})
	}

	return c.JSON(http.StatusOK, txn)
}

func RegisterRoutes(e *echo.Echo, h *Handler) {
	api := e.Group("/api/v1")
	api.POST("/transfers", h.InitiateTransfer)
	api.GET("/transactions/:id", h.GetTransaction)
}
