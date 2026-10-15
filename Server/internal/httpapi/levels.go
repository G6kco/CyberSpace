package httpapi

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/levels"
)

// uploadWindow is how long one file upload may take. The server's read and
// write timeouts suit ordinary requests, not a large file over a slow link.
const uploadWindow = 15 * time.Minute

type levelHandler struct {
	service *levels.Service
	logger  *zap.Logger
}

func registerLevelRoutes(protected *gin.RouterGroup, handler *levelHandler) {
	admin := protected.Group("/admin/levels", requireRole("admin"))
	admin.GET("", handler.list)
	admin.POST("", handler.create)
	admin.GET("/:levelID", handler.get)
	admin.PUT("/:levelID", handler.update)
	admin.DELETE("/:levelID", handler.delete)
	admin.POST("/:levelID/publish", handler.publish)
	admin.POST("/:levelID/unpublish", handler.unpublish)
	admin.POST("/:levelID/questions", handler.createQuestion)
	admin.PUT("/:levelID/questions/:questionID", handler.updateQuestion)
	admin.DELETE("/:levelID/questions/:questionID", handler.deleteQuestion)
	admin.POST("/:levelID/questions/:questionID/files", handler.uploadFile)
	admin.GET("/:levelID/questions/:questionID/files/:fileID", handler.downloadFile)
	admin.DELETE("/:levelID/questions/:questionID/files/:fileID", handler.deleteFile)
}

func (h *levelHandler) list(c *gin.Context) {
	list, err := h.service.List(c.Request.Context())
	h.respond(c, gin.H{"levels": list}, err)
}

func (h *levelHandler) get(c *gin.Context) {
	levelID, ok := idParam(c, "levelID")
	if !ok {
		return
	}
	level, err := h.service.Get(c.Request.Context(), levelID)
	h.respond(c, level, err)
}

func (h *levelHandler) create(c *gin.Context) {
	var in levels.LevelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "Body must be the level's settings")
		return
	}
	id, err := h.service.Create(c.Request.Context(), actorOf(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusCreated, id)
}

func (h *levelHandler) update(c *gin.Context) {
	levelID, ok := idParam(c, "levelID")
	if !ok {
		return
	}
	var in levels.LevelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "Body must be the level's settings")
		return
	}
	if err := h.service.Update(c.Request.Context(), actorOf(c), levelID, in); err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusOK, levelID)
}

func (h *levelHandler) delete(c *gin.Context) {
	levelID, ok := idParam(c, "levelID")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), actorOf(c), levelID); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *levelHandler) publish(c *gin.Context) {
	levelID, ok := idParam(c, "levelID")
	if !ok {
		return
	}
	if err := h.service.Publish(c.Request.Context(), actorOf(c), levelID); err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusOK, levelID)
}

func (h *levelHandler) unpublish(c *gin.Context) {
	levelID, ok := idParam(c, "levelID")
	if !ok {
		return
	}
	if err := h.service.Unpublish(c.Request.Context(), actorOf(c), levelID); err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusOK, levelID)
}

func (h *levelHandler) createQuestion(c *gin.Context) {
	levelID, ok := idParam(c, "levelID")
	if !ok {
		return
	}
	var in levels.QuestionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "Body must be {\"prompt\", \"image\", \"answers\": [...]}")
		return
	}
	if _, err := h.service.CreateQuestion(c.Request.Context(), actorOf(c), levelID, in); err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusCreated, levelID)
}

func (h *levelHandler) updateQuestion(c *gin.Context) {
	levelID, questionID, ok := levelAndQuestion(c)
	if !ok {
		return
	}
	var in levels.QuestionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "Body must be {\"prompt\", \"image\", \"answers\": [...]}")
		return
	}
	if err := h.service.UpdateQuestion(c.Request.Context(), actorOf(c), levelID, questionID, in); err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusOK, levelID)
}

func (h *levelHandler) deleteQuestion(c *gin.Context) {
	levelID, questionID, ok := levelAndQuestion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteQuestion(c.Request.Context(), actorOf(c), levelID, questionID); err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusOK, levelID)
}

// uploadFile streams the multipart field "file" to disk without buffering
// it in memory.
func (h *levelHandler) uploadFile(c *gin.Context) {
	levelID, questionID, ok := levelAndQuestion(c)
	if !ok {
		return
	}
	controller := http.NewResponseController(c.Writer)
	_ = controller.SetReadDeadline(time.Now().Add(uploadWindow))
	_ = controller.SetWriteDeadline(time.Now().Add(uploadWindow + time.Minute))
	// The multipart framing around the file is small; a megabyte covers it.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.service.MaxFileBytes()+1<<20)

	reader, err := c.Request.MultipartReader()
	if err != nil {
		badRequest(c, "Send the file as multipart/form-data in the field \"file\"")
		return
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			badRequest(c, "Send the file as multipart/form-data in the field \"file\"")
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
		_, err = h.service.UploadFile(c.Request.Context(), actorOf(c), levelID, questionID, part.FileName(), part)
		_ = part.Close()
		if err != nil {
			h.fail(c, err)
			return
		}
		h.reply(c, http.StatusCreated, levelID)
		return
	}
}

func (h *levelHandler) downloadFile(c *gin.Context) {
	levelID, questionID, ok := levelAndQuestion(c)
	if !ok {
		return
	}
	fileID, ok := idParam(c, "fileID")
	if !ok {
		return
	}
	path, name, err := h.service.FilePath(c.Request.Context(), levelID, questionID, fileID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.FileAttachment(path, name)
}

func (h *levelHandler) deleteFile(c *gin.Context) {
	levelID, questionID, ok := levelAndQuestion(c)
	if !ok {
		return
	}
	fileID, ok := idParam(c, "fileID")
	if !ok {
		return
	}
	if err := h.service.DeleteFile(c.Request.Context(), actorOf(c), levelID, questionID, fileID); err != nil {
		h.fail(c, err)
		return
	}
	h.reply(c, http.StatusOK, levelID)
}

// reply answers a change with the level as it now stands, so the editor
// redraws from one response.
func (h *levelHandler) reply(c *gin.Context, status int, levelID uint64) {
	level, err := h.service.Get(c.Request.Context(), levelID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(status, level)
}

func (h *levelHandler) respond(c *gin.Context, body any, err error) {
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, body)
}

// fail maps builder errors to responses. Unknown errors are logged and
// reported generically.
func (h *levelHandler) fail(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, levels.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, levels.ErrInvalidInput):
		status, code = http.StatusBadRequest, "invalid_input"
	case errors.Is(err, levels.ErrPublished):
		status, code = http.StatusConflict, "level_published"
	case errors.Is(err, levels.ErrReadOnly):
		status, code = http.StatusConflict, "level_read_only"
	case errors.Is(err, levels.ErrInUse):
		status, code = http.StatusConflict, "level_in_use"
	case errors.Is(err, levels.ErrNotReady):
		status, code = http.StatusConflict, "level_not_ready"
	case errors.Is(err, levels.ErrTooLarge), errors.As(err, &tooLarge):
		status, code = http.StatusRequestEntityTooLarge, "file_too_large"
		err = levels.ErrTooLarge
	}
	if status == http.StatusInternalServerError {
		h.logger.Error("Level builder request failed",
			zap.String("path", c.FullPath()), zap.Error(err))
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": levels.Message(err)})
}

func levelAndQuestion(c *gin.Context) (uint64, uint64, bool) {
	levelID, ok := idParam(c, "levelID")
	if !ok {
		return 0, 0, false
	}
	questionID, ok := idParam(c, "questionID")
	return levelID, questionID, ok
}
