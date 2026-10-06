package authtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/G6kco/CyberSpace/internal/auth"
)

// UserRecord is a users row as the memory repository stores it.
type UserRecord struct {
	auth.User
	Status        string
	GoogleSubject string
}

// SessionRecord is an auth_sessions row as the memory repository stores it.
type SessionRecord struct {
	UserID    uint64
	ExpiresAt time.Time
	Revoked   bool
}

type flowRecord struct {
	flow     auth.LoginFlow
	consumed bool
}

// MemoryRepository implements auth.Repository with the same observable rules
// as database.AuthRepository, so the auth service can be tested without MySQL.
// The SQL implementation itself is covered by the database package's
// integration tests.
type MemoryRepository struct {
	mu       sync.Mutex
	users    []*UserRecord
	flows    map[[32]byte]*flowRecord
	sessions map[[32]byte]*SessionRecord

	// Fail maps a method name, such as "CreateSession", to the error that
	// method returns, simulating a database fault at that step.
	Fail map[string]error
}

var _ auth.Repository = (*MemoryRepository)(nil)

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		flows:    map[[32]byte]*flowRecord{},
		sessions: map[[32]byte]*SessionRecord{},
		Fail:     map[string]error{},
	}
}

// AddUser provisions an account, as an administrator would.
func (r *MemoryRepository) AddUser(email, role, status string) *UserRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := uint64(len(r.users) + 1)
	user := &UserRecord{
		User: auth.User{
			ID:       id,
			PublicID: fmt.Sprintf("01USER%020d", id),
			Name:     "User " + email,
			Email:    email,
			Role:     role,
		},
		Status: status,
	}
	r.users = append(r.users, user)
	return user
}

// SetStatus changes an account's status, as an administrator would.
func (r *MemoryRepository) SetStatus(user *UserRecord, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user.Status = status
}

// ExpireFlows moves every login flow past its expiry.
func (r *MemoryRepository) ExpireFlows() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.flows {
		record.flow.ExpiresAt = time.Now().Add(-time.Second)
	}
}

// ExpireSessions moves every session past its expiry.
func (r *MemoryRepository) ExpireSessions() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, session := range r.sessions {
		session.ExpiresAt = time.Now().Add(-time.Second)
	}
}

// Flows returns a copy of every stored login flow.
func (r *MemoryRepository) Flows() []auth.LoginFlow {
	r.mu.Lock()
	defer r.mu.Unlock()
	flows := make([]auth.LoginFlow, 0, len(r.flows))
	for _, record := range r.flows {
		flows = append(flows, record.flow)
	}
	return flows
}

// Session returns a copy of the session stored under tokenHash.
func (r *MemoryRepository) Session(tokenHash [32]byte) (SessionRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[tokenHash]
	if !ok {
		return SessionRecord{}, false
	}
	return *session, true
}

// SessionCount returns how many sessions have been created.
func (r *MemoryRepository) SessionCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions)
}

func (r *MemoryRepository) SaveFlow(_ context.Context, flow auth.LoginFlow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Fail["SaveFlow"]; err != nil {
		return err
	}
	r.flows[flow.StateHash] = &flowRecord{flow: flow}
	return nil
}

func (r *MemoryRepository) ConsumeFlow(
	_ context.Context,
	stateHash, browserHash [32]byte,
) (auth.LoginFlow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Fail["ConsumeFlow"]; err != nil {
		return auth.LoginFlow{}, err
	}

	record, ok := r.flows[stateHash]
	if !ok ||
		record.flow.BrowserHash != browserHash ||
		record.consumed ||
		!record.flow.ExpiresAt.After(time.Now()) {
		return auth.LoginFlow{}, auth.ErrInvalidFlow
	}
	record.consumed = true
	return record.flow, nil
}

func (r *MemoryRepository) FindOrBindUser(
	_ context.Context,
	subject, email string,
) (auth.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Fail["FindOrBindUser"]; err != nil {
		return auth.User{}, err
	}

	var found *UserRecord
	for _, user := range r.users {
		if user.GoogleSubject == subject {
			found = user
		}
	}
	if found == nil {
		for _, user := range r.users {
			// The users.email column uses a case-insensitive collation.
			if strings.EqualFold(user.Email, email) {
				found = user
			}
		}
	}

	switch {
	case found == nil:
		return auth.User{}, fmt.Errorf("%w: no user row for email %q", auth.ErrAccessDenied, email)
	case found.Status != "active":
		return auth.User{}, fmt.Errorf("%w: user has status %q", auth.ErrAccessDenied, found.Status)
	case found.GoogleSubject != "" && found.GoogleSubject != subject:
		return auth.User{}, fmt.Errorf("%w: bound to a different Google account", auth.ErrAccessDenied)
	}

	found.GoogleSubject = subject
	return found.User, nil
}

func (r *MemoryRepository) CreateSession(
	_ context.Context,
	userID uint64,
	tokenHash [32]byte,
	expiresAt time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Fail["CreateSession"]; err != nil {
		return err
	}
	r.sessions[tokenHash] = &SessionRecord{UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (r *MemoryRepository) ResolveSession(
	_ context.Context,
	tokenHash [32]byte,
) (auth.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Fail["ResolveSession"]; err != nil {
		return auth.User{}, err
	}

	session, ok := r.sessions[tokenHash]
	if !ok || session.Revoked || !session.ExpiresAt.After(time.Now()) {
		return auth.User{}, auth.ErrUnauthenticated
	}
	for _, user := range r.users {
		if user.ID == session.UserID && user.Status == "active" {
			return user.User, nil
		}
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (r *MemoryRepository) RevokeSession(
	_ context.Context,
	tokenHash [32]byte,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Fail["RevokeSession"]; err != nil {
		return err
	}
	if session, ok := r.sessions[tokenHash]; ok {
		session.Revoked = true
	}
	return nil
}
