package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/viabtc/go-project/services/alertcenter/internal/alerter"
)

type Handler struct {
	alerter *alerter.Alerter
}

func New(a *alerter.Alerter) *Handler {
	return &Handler{alerter: a}
}

func (h *Handler) HandleAlert(c *gin.Context) {
	message, exists := c.Get("alert_message")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no alert message"})
		return
	}

	if err := h.alerter.SendAlert(c.Request.Context(), message.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) HandleGetAlerts(c *gin.Context) {
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "100"), 10, 64)

	alerts, err := h.alerter.GetAlerts(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func (h *Handler) HandleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
