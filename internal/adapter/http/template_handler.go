package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mehmetymw/event-driven-ns/internal/app"
)

type TemplateHandler struct {
	service *app.TemplateService
}

func NewTemplateHandler(service *app.TemplateService) *TemplateHandler {
	return &TemplateHandler{service: service}
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	tmpl, err := h.service.Create(c.Request.Context(), req.ToInput())
	if err != nil {
		handleDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, NewTemplateResponse(tmpl))
}

func (h *TemplateHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid template id"})
		return
	}

	tmpl, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		handleDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, NewTemplateResponse(tmpl))
}

func (h *TemplateHandler) List(c *gin.Context) {
	templates, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	data := make([]TemplateResponse, len(templates))
	for i, t := range templates {
		data[i] = NewTemplateResponse(t)
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}
