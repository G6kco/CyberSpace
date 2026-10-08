package assessments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/evaluation"
	"github.com/G6kco/CyberSpace/internal/labs"
)

// Config holds the assessment rules. Zero durations and limits are replaced
// by the defaults decided for Level 1.
type Config struct {
	// AnswerKey keys the stored flag digests; see evaluation.Digest.
	AnswerKey []byte
	// PcapDir holds the Wireshark capture files.
	PcapDir string
	// Cooldown is how long a student who did not pass waits to retake.
	Cooldown time.Duration
	// AttendanceWindow is how long taken attendance stays valid for a start.
	AttendanceWindow time.Duration
	// PresenceTTL is how long a portal heartbeat keeps a student listed.
	PresenceTTL time.Duration
	// SubmissionsPerMinute caps flag submissions per attempt.
	SubmissionsPerMinute int
	// MaxAnswerLength bounds a submitted answer, in characters.
	MaxAnswerLength int
}

func (c *Config) applyDefaults() {
	if c.Cooldown <= 0 {
		c.Cooldown = 24 * time.Hour
	}
	if c.AttendanceWindow <= 0 {
		c.AttendanceWindow = 4 * time.Hour
	}
	if c.PresenceTTL <= 0 {
		c.PresenceTTL = 30 * time.Second
	}
	if c.SubmissionsPerMinute <= 0 {
		c.SubmissionsPerMinute = 20
	}
	if c.MaxAnswerLength <= 0 {
		c.MaxAnswerLength = 256
	}
}

// Service applies the assessment rules over storage and the lab engine.
// Portal presence and submission rate limits live in memory, which is
// correct for the single API process this platform runs as.
type Service struct {
	repo     Repository
	labs     Labs
	logger   *zap.Logger
	cfg      Config
	now      func() time.Time
	presence *presence
	limiter  *limiter
}

func NewService(repo Repository, labs Labs, logger *zap.Logger, cfg Config) (*Service, error) {
	cfg.applyDefaults()
	if len(cfg.AnswerKey) < evaluation.MinKeyLength {
		return nil, fmt.Errorf("assessments: %w", evaluation.ErrShortKey)
	}
	return &Service{
		repo:     repo,
		labs:     labs,
		logger:   logger,
		cfg:      cfg,
		now:      func() time.Time { return time.Now().UTC() },
		presence: newPresence(),
		limiter:  newLimiter(cfg.SubmissionsPerMinute, time.Minute),
	}, nil
}

// ---- Student ---------------------------------------------------------------

// Overview reports what the student may do now.
func (s *Service) Overview(ctx context.Context, userID uint64) (Overview, error) {
	overview, _, err := s.evaluate(ctx, userID, s.now())
	return overview, err
}

// Heartbeat records the student as present in the test portal when they are
// eligible, and returns their overview; the portal polls it, so a started
// attempt appears in the response.
func (s *Service) Heartbeat(ctx context.Context, userID uint64) (Overview, error) {
	now := s.now()
	overview, _, err := s.evaluate(ctx, userID, now)
	if err != nil {
		return Overview{}, err
	}
	if overview.Status == StatusEligible {
		s.presence.touch(userID, now)
		overview.InLobby = true
	} else {
		s.presence.leave(userID)
		overview.InLobby = false
	}
	return overview, nil
}

// LeaveLobby removes the student from the portal list.
func (s *Service) LeaveLobby(userID uint64) {
	s.presence.leave(userID)
}

// Attempt returns the student's own attempt.
func (s *Service) Attempt(ctx context.Context, userID, attemptID uint64) (AttemptView, error) {
	view, owner, err := s.repo.Attempt(ctx, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	if owner != userID {
		return AttemptView{}, ErrNotFound
	}
	view.ServerTime = s.now()
	if view.Status != "in_progress" {
		// The questions and lab belong to the sitting, not the record.
		view.Questions, view.Lab = nil, nil
		return view, nil
	}
	for i := range view.Questions {
		view.Questions[i].HasPcap = view.Questions[i].PcapFile != ""
	}
	return view, nil
}

// Submit checks one flag answer.
func (s *Service) Submit(ctx context.Context, userID, attemptID, flagID uint64, answer string) (SubmitResult, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" || utf8.RuneCountInString(answer) > s.cfg.MaxAnswerLength {
		return SubmitResult{}, fmt.Errorf("%w: answer must be 1 to %d characters", ErrInvalidInput, s.cfg.MaxAnswerLength)
	}
	now := s.now()
	if !s.limiter.allow(attemptID, now) {
		return SubmitResult{}, ErrTooManySubmissions
	}
	return s.repo.SubmitAnswer(ctx, attemptID, userID, flagID, evaluation.Digest(s.cfg.AnswerKey, answer), now)
}

// Finish submits the student's attempt for scoring and stops its lab.
func (s *Service) Finish(ctx context.Context, userID, attemptID uint64) (AttemptSummary, error) {
	_, owner, err := s.repo.Attempt(ctx, attemptID)
	if err != nil {
		return AttemptSummary{}, err
	}
	if owner != userID {
		return AttemptSummary{}, ErrNotFound
	}
	return s.finish(ctx, attemptID, userID, FinishSubmit, "")
}

// Pcap returns the capture file of a Wireshark question dealt to the
// student's attempt, only while the attempt is in progress.
func (s *Service) Pcap(ctx context.Context, userID, attemptID, questionID uint64) (path, name string, err error) {
	view, err := s.Attempt(ctx, userID, attemptID)
	if err != nil {
		return "", "", err
	}
	if view.Status != "in_progress" {
		return "", "", ErrAttemptClosed
	}
	for _, q := range view.Questions {
		if q.ID != questionID || q.Pool != "wireshark" || q.PcapFile == "" {
			continue
		}
		// pcap_file is a bare name by CHECK constraint; Base is a second
		// guard so no stored value can leave the capture directory.
		name := filepath.Base(q.PcapFile)
		path := filepath.Join(s.cfg.PcapDir, name)
		if _, err := os.Stat(path); err != nil {
			s.logger.Error("Capture file missing", zap.String("file", path), zap.Error(err))
			return "", "", fmt.Errorf("%w: capture file unavailable", ErrNotFound)
		}
		return path, name, nil
	}
	return "", "", ErrNotFound
}

// ---- Administrator ---------------------------------------------------------

// ActionResult reports one student's outcome of a bulk action.
type ActionResult struct {
	StudentID string  `json:"studentId"`
	OK        bool    `json:"ok"`
	AttemptID *uint64 `json:"attemptId,omitempty"`
	Error     string  `json:"error,omitempty"`
	Warning   string  `json:"warning,omitempty"`
}

// Lobby lists students in the portal and students whose attendance is taken.
func (s *Service) Lobby(ctx context.Context) ([]LobbyEntry, error) {
	now := s.now()
	online := s.presence.online(now, s.cfg.PresenceTTL)
	attendance, err := s.repo.AttendanceUsers(ctx, now)
	if err != nil {
		return nil, err
	}

	ids := make([]uint64, 0, len(online)+len(attendance))
	for id := range online {
		ids = append(ids, id)
	}
	for id := range attendance {
		if _, ok := online[id]; !ok {
			ids = append(ids, id)
		}
	}
	students, err := s.repo.Students(ctx, ids)
	if err != nil {
		return nil, err
	}

	entries := make([]LobbyEntry, 0, len(students))
	for _, student := range students {
		overview, _, err := s.evaluate(ctx, student.ID, now)
		if err != nil {
			return nil, err
		}
		entry := LobbyEntry{
			Student:          student,
			AttendanceMarked: overview.AttendanceMarked,
			ActiveAttemptID:  overview.ActiveAttemptID,
			Eligibility:      overview.Status,
		}
		if overview.Assessment != nil {
			entry.Level = overview.Assessment.LevelName
		}
		if seen, ok := online[student.ID]; ok {
			entry.Online = true
			entry.LastSeen = &seen
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// MarkAttendance takes attendance for students present in the portal.
func (s *Service) MarkAttendance(ctx context.Context, actor Actor, studentIDs []string) []ActionResult {
	return s.each(ctx, actor, "attendance.mark", studentIDs, func(student Student, now time.Time) (ActionResult, error) {
		if !s.presence.isOnline(student.ID, now, s.cfg.PresenceTTL) {
			return ActionResult{}, ErrNotInLobby
		}
		overview, next, err := s.evaluate(ctx, student.ID, now)
		if err != nil {
			return ActionResult{}, err
		}
		if overview.Status != StatusEligible {
			return ActionResult{}, fmt.Errorf("%w: %s", ErrNotEligible, overview.Status)
		}
		err = s.repo.MarkAttendance(ctx, actor.UserID, student.ID, next.VersionID, now, now.Add(s.cfg.AttendanceWindow))
		return ActionResult{}, err
	})
}

// CancelAttendance withdraws a student's taken attendance.
func (s *Service) CancelAttendance(ctx context.Context, actor Actor, studentID string) (ActionResult, error) {
	results := s.each(ctx, actor, "attendance.cancel", []string{studentID}, func(student Student, now time.Time) (ActionResult, error) {
		n, err := s.repo.CancelAttendance(ctx, actor.UserID, student.ID, now, "Attendance withdrawn by administrator")
		if err == nil && n == 0 {
			err = ErrNoAttendance
		}
		return ActionResult{}, err
	})
	return results[0], nil
}

// Start starts the assessment for students whose attendance is taken: each
// is dealt their questions and their lab is queued.
func (s *Service) Start(ctx context.Context, actor Actor, studentIDs []string) []ActionResult {
	return s.each(ctx, actor, "attempt.start", studentIDs, func(student Student, now time.Time) (ActionResult, error) {
		overview, next, err := s.evaluate(ctx, student.ID, now)
		if err != nil {
			return ActionResult{}, err
		}
		if overview.Status != StatusEligible {
			return ActionResult{}, fmt.Errorf("%w: %s", ErrNotEligible, overview.Status)
		}
		if !overview.AttendanceMarked {
			return ActionResult{}, ErrNoAttendance
		}

		started, err := s.repo.StartAttempt(ctx, StartRequest{
			AdminID:         actor.UserID,
			UserID:          student.ID,
			VersionID:       next.VersionID,
			DurationSeconds: next.DurationSeconds,
			MaxScore:        next.TotalPoints,
			Now:             now,
		})
		if err != nil {
			return ActionResult{}, err
		}
		s.presence.leave(student.ID)

		result := ActionResult{AttemptID: &started.AttemptID}
		if started.LabTemplateVersionID != 0 {
			// The attempt stands even if no lab could be queued: the
			// administrator sees the warning and can restart the lab.
			if _, err := s.labs.RequestProvision(ctx, started.AttemptID, started.LabTemplateVersionID); err != nil {
				s.logger.Error("Lab request failed", zap.Uint64("attempt", started.AttemptID), zap.Error(err))
				result.Warning = "lab could not be queued: " + err.Error()
			}
		}
		return result, nil
	})
}

// Attempts lists attempts for monitoring: in progress, or the latest ones.
func (s *Service) Attempts(ctx context.Context, active bool) ([]AdminAttempt, error) {
	return s.repo.ListAttempts(ctx, active, 200)
}

// AdminAttempt returns any attempt with its questions and lab.
func (s *Service) AdminAttempt(ctx context.Context, attemptID uint64) (AttemptView, error) {
	view, _, err := s.repo.Attempt(ctx, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	view.ServerTime = s.now()
	return view, nil
}

// End ends an attempt early; it is scored as it stands.
func (s *Service) End(ctx context.Context, actor Actor, attemptID uint64) (AttemptSummary, error) {
	summary, err := s.finish(ctx, attemptID, actor.UserID, FinishEnd, "Ended by administrator")
	s.audit(ctx, actor, "attempt.end", "attempt", strconv.FormatUint(attemptID, 10), summary, err)
	return summary, err
}

// Revoke revokes an attempt; it is not scored and counts as not passed.
func (s *Service) Revoke(ctx context.Context, actor Actor, attemptID uint64, reason string) (AttemptSummary, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		return AttemptSummary{}, fmt.Errorf("%w: a reason of up to 500 characters is required", ErrInvalidInput)
	}
	summary, err := s.finish(ctx, attemptID, actor.UserID, FinishRevoke, reason)
	s.audit(ctx, actor, "attempt.revoke", "attempt", strconv.FormatUint(attemptID, 10), summary, err)
	return summary, err
}

// RestartLab queues a fresh lab for an in-progress attempt whose lab failed
// or stopped.
func (s *Service) RestartLab(ctx context.Context, actor Actor, attemptID uint64) error {
	err := s.restartLab(ctx, attemptID)
	s.audit(ctx, actor, "lab.restart", "attempt", strconv.FormatUint(attemptID, 10), nil, err)
	return err
}

func (s *Service) restartLab(ctx context.Context, attemptID uint64) error {
	view, _, err := s.repo.Attempt(ctx, attemptID)
	if err != nil {
		return err
	}
	if view.Status != "in_progress" {
		return ErrAttemptClosed
	}
	templateID, err := s.repo.AttemptLab(ctx, attemptID)
	if err != nil {
		return err
	}
	if templateID == 0 {
		return ErrNoLab
	}
	_, err = s.labs.RequestProvision(ctx, attemptID, templateID)
	if errors.Is(err, labs.ErrLabActive) {
		return ErrLabActive
	}
	return err
}

// QuestionBank lists every published assessment's questions, reporting
// whether each capture file is present on disk.
func (s *Service) QuestionBank(ctx context.Context) ([]BankAssessment, error) {
	bank, err := s.repo.QuestionBank(ctx)
	if err != nil {
		return nil, err
	}
	for i := range bank {
		for j := range bank[i].Questions {
			q := &bank[i].Questions[j]
			if q.PcapFile != "" {
				_, statErr := os.Stat(filepath.Join(s.cfg.PcapDir, filepath.Base(q.PcapFile)))
				q.PcapPresent = statErr == nil
			}
		}
	}
	return bank, nil
}

// StudentRecords lists students with their progress.
func (s *Service) StudentRecords(ctx context.Context, search string) ([]StudentRecord, error) {
	records, err := s.repo.StudentRecords(ctx, strings.TrimSpace(search), 500)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range records {
		last := records[i].LastAttempt
		if last != nil && last.EndedAt != nil && last.Result != "passed" && last.Status != "in_progress" {
			until := last.EndedAt.Add(s.cfg.Cooldown)
			if until.After(now) {
				records[i].CooldownUntil = &until
			}
		}
	}
	return records, nil
}

// RunExpiry scores attempts whose deadline has passed, every interval,
// until ctx is cancelled. It blocks.
func (s *Service) RunExpiry(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.ExpireDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ExpireDue scores every attempt past its deadline once.
func (s *Service) ExpireDue(ctx context.Context) {
	due, err := s.repo.DueAttempts(ctx, s.now())
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Error("Listing expired attempts failed", zap.Error(err))
		}
		return
	}
	for _, id := range due {
		if _, err := s.finish(ctx, id, 0, FinishExpire, "Time expired"); err != nil && !errors.Is(err, ErrAttemptClosed) {
			s.logger.Error("Expiring attempt failed", zap.Uint64("attempt", id), zap.Error(err))
		}
	}
}

// ---- Shared ----------------------------------------------------------------

// evaluate works out the student's next assessment and status.
func (s *Service) evaluate(ctx context.Context, userID uint64, now time.Time) (Overview, *Assessment, error) {
	catalog, err := s.repo.Catalog(ctx)
	if err != nil {
		return Overview{}, nil, err
	}
	state, err := s.repo.StudentState(ctx, userID, now)
	if err != nil {
		return Overview{}, nil, err
	}

	overview := Overview{
		History:    state.History,
		InLobby:    s.presence.isOnline(userID, now, s.cfg.PresenceTTL),
		ServerTime: now,
	}
	if overview.History == nil {
		overview.History = []AttemptSummary{}
	}

	var next *Assessment
	for i := range catalog {
		if !state.PassedLevels[catalog[i].LevelID] {
			next = &catalog[i]
			break
		}
	}
	overview.Assessment = next

	switch {
	case state.ActiveAttemptID != 0:
		overview.Status = StatusInProgress
		id := state.ActiveAttemptID
		overview.ActiveAttemptID = &id
	case len(catalog) == 0:
		overview.Status = StatusUnavailable
	case next == nil:
		overview.Status = StatusCompleted
	default:
		overview.Status = StatusEligible
		if ended, ok := state.LastFailure[next.LevelID]; ok {
			if until := ended.Add(s.cfg.Cooldown); until.After(now) {
				overview.Status = StatusCooldown
				overview.CooldownUntil = &until
			}
		}
		overview.AttendanceMarked = state.AttendanceVersions[next.VersionID]
	}
	return overview, next, nil
}

// finish closes an attempt and stops its lab. The lab stop is also queued by
// the engine's sweep, so a failure here only delays it.
func (s *Service) finish(ctx context.Context, attemptID, actorID uint64, mode, reason string) (AttemptSummary, error) {
	summary, err := s.repo.FinishAttempt(ctx, attemptID, actorID, mode, reason, s.now())
	if err != nil {
		return AttemptSummary{}, err
	}
	if err := s.labs.RequestStop(ctx, attemptID); err != nil {
		s.logger.Error("Lab stop request failed", zap.Uint64("attempt", attemptID), zap.Error(err))
	}
	return summary, nil
}

// each runs a per-student administrative action, auditing every outcome.
func (s *Service) each(
	ctx context.Context,
	actor Actor,
	action string,
	studentIDs []string,
	run func(Student, time.Time) (ActionResult, error),
) []ActionResult {
	results := make([]ActionResult, 0, len(studentIDs))
	seen := map[string]bool{}
	for _, publicID := range studentIDs {
		if seen[publicID] {
			continue
		}
		seen[publicID] = true

		result := ActionResult{StudentID: publicID}
		student, err := s.repo.StudentByPublicID(ctx, publicID)
		if err == nil {
			var outcome ActionResult
			outcome, err = run(student, s.now())
			result.AttemptID, result.Warning = outcome.AttemptID, outcome.Warning
		}
		if err != nil {
			result.Error = Message(err)
		} else {
			result.OK = true
		}
		s.audit(ctx, actor, action, "user", publicID, result, err)
		results = append(results, result)
	}
	return results
}

func (s *Service) audit(ctx context.Context, actor Actor, action, entityType, entityID string, after any, err error) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
		after = map[string]string{"error": err.Error()}
	}
	if auditErr := s.repo.Audit(ctx, AuditEntry{
		Actor:      actor,
		Action:     action,
		Outcome:    outcome,
		EntityType: entityType,
		EntityID:   entityID,
		After:      after,
	}); auditErr != nil {
		s.logger.Error("Audit write failed", zap.String("action", action), zap.Error(auditErr))
	}
}

// Message is the user-facing text for a domain error.
func Message(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "Not found."
	case errors.Is(err, ErrNotInLobby):
		return "The student is not in the test portal."
	case errors.Is(err, ErrNoAttendance):
		return "Attendance has not been taken for this student."
	case errors.Is(err, ErrNotEligible):
		return "The student is not eligible (" + strings.TrimPrefix(err.Error(), ErrNotEligible.Error()+": ") + ")."
	case errors.Is(err, ErrAttemptClosed):
		return "The attempt is no longer in progress."
	case errors.Is(err, ErrAlreadySolved):
		return "This flag is already solved."
	case errors.Is(err, ErrTooManySubmissions):
		return "Too many submissions. Wait a moment and try again."
	case errors.Is(err, ErrNoLab):
		return "This attempt has no lab."
	case errors.Is(err, ErrLabActive):
		return "The lab is still running or starting."
	case errors.Is(err, labs.ErrNoFreeAddress):
		return "No lab address is free. Free capacity or add addresses, then restart the lab."
	case errors.Is(err, ErrInvalidInput):
		return strings.TrimPrefix(err.Error(), ErrInvalidInput.Error()+": ")
	default:
		return "The action failed."
	}
}
