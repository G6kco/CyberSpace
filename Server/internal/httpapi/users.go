package httpapi

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/users"
)

type userHandler struct {
	service *users.Service
	logger  *zap.Logger
}

func registerUserRoutes(protected *gin.RouterGroup, handler *userHandler) {
	admin := protected.Group("/admin", requireRole("admin"))
	admin.POST("/students/import", handler.importStudents)
	admin.GET("/admins", handler.admins)
	admin.POST("/admins", handler.createAdmin)
}

// importStudents reads the multipart field "file", an .xlsx student list.
// The workbook is parsed in memory, so the body is bounded first.
func (h *userHandler) importStudents(c *gin.Context) {
	controller := http.NewResponseController(c.Writer)
	_ = controller.SetReadDeadline(time.Now().Add(uploadWindow))
	_ = controller.SetWriteDeadline(time.Now().Add(uploadWindow + time.Minute))
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.service.MaxUploadBytes()+1<<20)

	reader, err := c.Request.MultipartReader()
	if err != nil {
		badRequest(c, "Send the workbook as multipart/form-data in the field \"file\"")
		return
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			badRequest(c, "Send the workbook as multipart/form-data in the field \"file\"")
			return
		}
		if err != nil {
			h.fail(c, err)
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		result, err := h.service.ImportStudents(c.Request.Context(), actorOf(c), part)
		_ = part.Close()
		if err != nil {
			h.fail(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}
}

func (h *userHandler) admins(c *gin.Context) {
	admins, err := h.service.Admins(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"admins": admins})
}

func (h *userHandler) createAdmin(c *gin.Context) {
	var in users.AdminInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "Body must be {\"name\", \"email\"}")
		return
	}
	admin, err := h.service.CreateAdmin(c.Request.Context(), actorOf(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, admin)
}

// fail maps account errors to responses. Unknown errors are logged and
// reported generically.
func (h *userHandler) fail(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, users.ErrInvalidInput):
		status, code = http.StatusBadRequest, "invalid_input"
	case errors.Is(err, users.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, users.ErrTooLarge), errors.As(err, &tooLarge):
		status, code = http.StatusRequestEntityTooLarge, "file_too_large"
		err = users.ErrTooLarge
	}
	if status == http.StatusInternalServerError {
		h.logger.Error("Account request failed",
			zap.String("path", c.FullPath()), zap.Error(err))
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": users.Message(err)})
}
