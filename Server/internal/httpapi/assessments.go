package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/G6kco/CyberSpace/internal/labs"
)

// maxBulkStudents bounds one attendance or start request.
const maxBulkStudents = 500

type assessmentHandler struct {
	service *assessments.Service
	logger  *zap.Logger
}

func registerAssessmentRoutes(protected *gin.RouterGroup, handler *assessmentHandler) {
	student := protected.Group("/student", requireRole("student"))
	student.GET("/assessment", handler.overview)
	student.POST("/lobby/heartbeat", handler.heartbeat)
	student.DELETE("/lobby", handler.leaveLobby)
	student.GET("/attempts/:attemptID", handler.attempt)
	student.POST("/attempts/:attemptID/answers", handler.submit)
	student.POST("/attempts/:attemptID/finish", handler.finish)
	student.GET("/attempts/:attemptID/questions/:questionID/pcap", handler.pcap)

	admin := protected.Group("/admin", requireRole("admin"))
	admin.GET("/lobby", handler.lobby)
	admin.POST("/lobby/attendance", handler.markAttendance)
	admin.DELETE("/lobby/attendance/:studentID", handler.cancelAttendance)
	admin.POST("/lobby/start", handler.start)
	admin.GET("/attempts", handler.attempts)
	admin.GET("/attempts/:attemptID", handler.adminAttempt)
	admin.POST("/attempts/:attemptID/end", handler.end)
	admin.POST("/attempts/:attemptID/revoke", handler.revoke)
	admin.POST("/attempts/:attemptID/lab/restart", handler.restartLab)
	admin.GET("/question-bank", handler.questionBank)
	admin.GET("/students", handler.students)
}

// requireRole admits only users of role; it runs after requireSession.
func requireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUser(c).Role != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "forbidden",
				"message": "You do not have access to this resource",
			})
			return
		}
		c.Next()
	}
}

// requireOrigin rejects state-changing requests from any origin but the
// frontend's. The session cookie is attached automatically, so this is the
// cross-site request forgery defence alongside SameSite=Lax.
func requireOrigin(frontendURL string) gin.HandlerFunc {
	allowed := originOf(frontendURL)
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if !strings.EqualFold(c.GetHeader("Origin"), allowed) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "bad_origin",
				"message": "Request origin not allowed",
			})
			return
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) auth.User {
	return c.MustGet("currentUser").(auth.User)
}

func actorOf(c *gin.Context) assessments.Actor {
	return assessments.Actor{
		UserID:    currentUser(c).ID,
		IP:        net.ParseIP(c.ClientIP()),
		UserAgent: c.Request.UserAgent(),
	}
}

// ---- Student ---------------------------------------------------------------

func (h *assessmentHandler) overview(c *gin.Context) {
	overview, err := h.service.Overview(c.Request.Context(), currentUser(c).ID)
	h.respond(c, overview, err)
}

func (h *assessmentHandler) heartbeat(c *gin.Context) {
	overview, err := h.service.Heartbeat(c.Request.Context(), currentUser(c).ID)
	h.respond(c, overview, err)
}

func (h *assessmentHandler) leaveLobby(c *gin.Context) {
	h.service.LeaveLobby(currentUser(c).ID)
	c.Status(http.StatusNoContent)
}

func (h *assessmentHandler) attempt(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	view, err := h.service.Attempt(c.Request.Context(), currentUser(c).ID, attemptID)
	h.respond(c, view, err)
}

func (h *assessmentHandler) submit(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	var body struct {
		QuestionID uint64 `json:"questionId"`
		Answer     string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.QuestionID == 0 {
		badRequest(c, "Body must be {\"questionId\": number, \"answer\": string}")
		return
	}
	result, err := h.service.Submit(c.Request.Context(), currentUser(c).ID, attemptID, body.QuestionID, body.Answer)
	h.respond(c, result, err)
}

func (h *assessmentHandler) finish(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	summary, err := h.service.Finish(c.Request.Context(), currentUser(c).ID, attemptID)
	h.respond(c, summary, err)
}

func (h *assessmentHandler) pcap(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	questionID, ok := idParam(c, "questionID")
	if !ok {
		return
	}
	path, name, err := h.service.Pcap(c.Request.Context(), currentUser(c).ID, attemptID, questionID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.FileAttachment(path, name)
}

// ---- Administrator ---------------------------------------------------------

func (h *assessmentHandler) lobby(c *gin.Context) {
	entries, err := h.service.Lobby(c.Request.Context())
	h.respond(c, gin.H{"students": entries}, err)
}

func (h *assessmentHandler) markAttendance(c *gin.Context) {
	ids, ok := studentIDs(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": h.service.MarkAttendance(c.Request.Context(), actorOf(c), ids)})
}

func (h *assessmentHandler) cancelAttendance(c *gin.Context) {
	result, err := h.service.CancelAttendance(c.Request.Context(), actorOf(c), c.Param("studentID"))
	h.respond(c, result, err)
}

func (h *assessmentHandler) start(c *gin.Context) {
	ids, ok := studentIDs(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": h.service.Start(c.Request.Context(), actorOf(c), ids)})
}

func (h *assessmentHandler) attempts(c *gin.Context) {
	scope := c.DefaultQuery("scope", "active")
	if scope != "active" && scope != "recent" {
		badRequest(c, "scope must be active or recent")
		return
	}
	list, err := h.service.Attempts(c.Request.Context(), scope == "active")
	h.respond(c, gin.H{"attempts": list}, err)
}

func (h *assessmentHandler) adminAttempt(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	view, err := h.service.AdminAttempt(c.Request.Context(), attemptID)
	h.respond(c, view, err)
}

func (h *assessmentHandler) end(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	summary, err := h.service.End(c.Request.Context(), actorOf(c), attemptID)
	h.respond(c, summary, err)
}

func (h *assessmentHandler) revoke(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "Body must be {\"reason\": string}")
		return
	}
	summary, err := h.service.Revoke(c.Request.Context(), actorOf(c), attemptID, body.Reason)
	h.respond(c, summary, err)
}

func (h *assessmentHandler) restartLab(c *gin.Context) {
	attemptID, ok := idParam(c, "attemptID")
	if !ok {
		return
	}
	if err := h.service.RestartLab(c.Request.Context(), actorOf(c), attemptID); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *assessmentHandler) questionBank(c *gin.Context) {
	bank, err := h.service.QuestionBank(c.Request.Context())
	h.respond(c, gin.H{"assessments": bank}, err)
}

func (h *assessmentHandler) students(c *gin.Context) {
	search := c.Query("search")
	if len(search) > 100 {
		badRequest(c, "search must be at most 100 characters")
		return
	}
	records, err := h.service.StudentRecords(c.Request.Context(), search)
	h.respond(c, gin.H{"students": records}, err)
}

// ---- Helpers ---------------------------------------------------------------

func (h *assessmentHandler) respond(c *gin.Context, body any, err error) {
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, body)
}

// fail maps domain errors to responses. Unknown errors are logged and
// reported generically, so no internal detail reaches the browser.
func (h *assessmentHandler) fail(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, assessments.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, assessments.ErrInvalidInput):
		status, code = http.StatusBadRequest, "invalid_input"
	case errors.Is(err, assessments.ErrTooManySubmissions):
		status, code = http.StatusTooManyRequests, "too_many_submissions"
	case errors.Is(err, assessments.ErrAttemptClosed):
		status, code = http.StatusConflict, "attempt_closed"
	case errors.Is(err, assessments.ErrAlreadySolved):
		status, code = http.StatusConflict, "already_solved"
	case errors.Is(err, assessments.ErrNotEligible):
		status, code = http.StatusConflict, "not_eligible"
	case errors.Is(err, assessments.ErrNoAttendance):
		status, code = http.StatusConflict, "no_attendance"
	case errors.Is(err, assessments.ErrNotInLobby):
		status, code = http.StatusConflict, "not_in_lobby"
	case errors.Is(err, assessments.ErrNoLab):
		status, code = http.StatusConflict, "no_lab"
	case errors.Is(err, assessments.ErrLabActive):
		status, code = http.StatusConflict, "lab_active"
	case errors.Is(err, labs.ErrNoFreeAddress):
		status, code = http.StatusServiceUnavailable, "no_lab_capacity"
	}
	if status == http.StatusInternalServerError {
		h.logger.Error("Assessment request failed",
			zap.String("path", c.FullPath()), zap.Error(err))
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": assessments.Message(err)})
}

func idParam(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "Not found."})
		return 0, false
	}
	return id, true
}

func studentIDs(c *gin.Context) ([]string, bool) {
	var body struct {
		StudentIDs []string `json:"studentIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.StudentIDs) == 0 || len(body.StudentIDs) > maxBulkStudents {
		badRequest(c, "Body must be {\"studentIds\": [...]} with 1 to 500 IDs")
		return nil, false
	}
	return body.StudentIDs, true
}

func badRequest(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": message})
}

func originOf(rawURL string) string {
	scheme, rest, ok := strings.Cut(rawURL, "://")
	if !ok {
		return rawURL
	}
	host, _, _ := strings.Cut(rest, "/")
	return scheme + "://" + host
}
