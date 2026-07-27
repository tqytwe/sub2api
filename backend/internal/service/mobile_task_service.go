package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/google/uuid"
)

const (
	mobileTaskDefaultPageSize = 20
	mobileTaskMaxPageSize     = 100
	mobileTaskMaxArtifacts    = 100
	mobileTaskMaxTextRunes    = 512
)

var (
	ErrMobileTaskNotFound              = errors.New("mobile task not found")
	ErrMobileTaskClientRequestConflict = errors.New("mobile task client request id conflicts with another task")
	ErrMobileTaskStoreUnavailable      = errors.New("mobile task store is unavailable")
)

var mobileTaskSensitiveKeys = []string{
	"api_key", "apiKey", "token", "accessToken", "refreshToken", "secret", "prompt", "chat", "messages", "content",
	"request", "response", "authorization", "cookie", "signed_url",
}

type MobileTaskCreateInput struct {
	Kind            MobileTaskKind
	Operation       string
	ClientRequestID string
	ParentTaskID    string
	Resource        *MobileTaskResource
}

type MobileTaskListFilter struct {
	Kind     MobileTaskKind
	Status   MobileTaskStatus
	Page     int
	PageSize int
}

type MobileTaskPage struct {
	Items    []MobileTask `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Pages    int          `json:"pages"`
}

type MobileTaskDeleteResult struct {
	ID        string    `json:"id"`
	Deleted   bool      `json:"deleted"`
	DeletedAt time.Time `json:"deleted_at"`
}

type MobileTaskTransitionInput struct {
	Status    MobileTaskStatus
	Progress  *int
	Resource  *MobileTaskResource
	Artifacts []MobileTaskArtifact
	Error     *MobileTaskError
}

type MobileTaskService struct {
	db      *sql.DB
	dialect string
	now     func() time.Time
	newID   func() string
}

func NewMobileTaskService(db *sql.DB, dialectName ...string) *MobileTaskService {
	dialect := ""
	if len(dialectName) > 0 {
		dialect = strings.ToLower(strings.TrimSpace(dialectName[0]))
	}
	if dialect == "" && db != nil {
		driverType := strings.ToLower(fmt.Sprintf("%T", db.Driver()))
		if strings.Contains(driverType, "postgres") || strings.Contains(driverType, "pq.") || strings.Contains(driverType, "pgx") {
			dialect = "postgres"
		}
	}
	return &MobileTaskService{db: db, dialect: dialect, now: time.Now, newID: uuid.NewString}
}

func (s *MobileTaskService) Create(ctx context.Context, userID int64, input MobileTaskCreateInput) (*MobileTask, error) {
	if err := s.ready(userID); err != nil {
		return nil, err
	}
	input.Operation = strings.TrimSpace(input.Operation)
	input.ClientRequestID = strings.TrimSpace(input.ClientRequestID)
	input.ParentTaskID = strings.TrimSpace(input.ParentTaskID)
	if input.ParentTaskID != "" {
		if _, err := uuid.Parse(input.ParentTaskID); err != nil {
			return nil, ErrMobileTaskInvalidID
		}
	}
	if existing, err := s.getByClientRequestID(ctx, userID, input.ClientRequestID); err == nil {
		if !sameMobileTaskCreate(existing, input) {
			return nil, ErrMobileTaskClientRequestConflict
		}
		return existing, nil
	} else if !errors.Is(err, ErrMobileTaskNotFound) {
		return nil, err
	}

	task, err := NewMobileTask(s.newID(), input.Kind, input.Operation, input.ClientRequestID, s.now())
	if err != nil {
		return nil, err
	}
	task.ParentTaskID = input.ParentTaskID
	task.Resource = sanitizeMobileTaskResource(input.Resource)
	if err := ValidateMobileTask(task); err != nil {
		return nil, err
	}
	if err := s.insert(ctx, userID, task); err != nil {
		existing, lookupErr := s.getByClientRequestID(ctx, userID, input.ClientRequestID)
		if lookupErr == nil {
			if !sameMobileTaskCreate(existing, input) {
				return nil, ErrMobileTaskClientRequestConflict
			}
			return existing, nil
		}
		return nil, err
	}
	return &task, nil
}

func (s *MobileTaskService) Get(ctx context.Context, userID int64, id string) (*MobileTask, error) {
	if err := s.ready(userID); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrMobileTaskNotFound
	}
	return s.queryOne(ctx, mobileTaskSelect+" WHERE user_id = ? AND id = ? AND deleted_at IS NULL", userID, id)
}

func (s *MobileTaskService) List(ctx context.Context, userID int64, filter MobileTaskListFilter) (*MobileTaskPage, error) {
	if err := s.ready(userID); err != nil {
		return nil, err
	}
	filter = normalizeMobileTaskListFilter(filter)
	where := []string{"user_id = ?", "deleted_at IS NULL"}
	args := []any{userID}
	if filter.Kind != "" {
		if !IsValidMobileTaskKind(filter.Kind) {
			return nil, ErrMobileTaskInvalidKind
		}
		where = append(where, "kind = ?")
		args = append(args, string(filter.Kind))
	}
	if filter.Status != "" {
		if !IsValidMobileTaskStatus(filter.Status) {
			return nil, ErrMobileTaskInvalidStatus
		}
		where = append(where, "status = ?")
		args = append(args, string(filter.Status))
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, s.bind("SELECT COUNT(*) FROM mobile_tasks WHERE "+clause), args...).Scan(&total); err != nil {
		return nil, err
	}

	queryArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, s.bind(mobileTaskSelect+" WHERE "+clause+" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?"), queryArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]MobileTask, 0)
	for rows.Next() {
		task, scanErr := scanMobileTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := 0
	if total > 0 {
		pages = int((total + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}
	return &MobileTaskPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, Pages: pages}, nil
}

func (s *MobileTaskService) Cancel(ctx context.Context, userID int64, id string) (*MobileTask, error) {
	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	cancelled, err := CancelMobileTask(*current, s.now())
	if err != nil {
		return nil, err
	}
	if current.Status == MobileTaskStatusCancelled {
		return current, nil
	}
	return s.persistTransition(ctx, userID, *current, cancelled)
}

func (s *MobileTaskService) Retry(ctx context.Context, userID int64, id, clientRequestID string) (*MobileTask, error) {
	if err := s.ready(userID); err != nil {
		return nil, err
	}
	if existing, err := s.getByClientRequestID(ctx, userID, strings.TrimSpace(clientRequestID)); err == nil {
		if existing.RetryOf != strings.TrimSpace(id) {
			return nil, ErrMobileTaskClientRequestConflict
		}
		return existing, nil
	} else if !errors.Is(err, ErrMobileTaskNotFound) {
		return nil, err
	}
	source, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	retry, err := RetryMobileTask(*source, s.newID(), strings.TrimSpace(clientRequestID), s.now())
	if err != nil {
		return nil, err
	}
	if err := s.insert(ctx, userID, retry); err != nil {
		existing, lookupErr := s.getByClientRequestID(ctx, userID, retry.ClientRequestID)
		if lookupErr == nil && existing.RetryOf == source.ID {
			return existing, nil
		}
		return nil, err
	}
	return &retry, nil
}

func (s *MobileTaskService) Delete(ctx context.Context, userID int64, id string) (*MobileTaskDeleteResult, error) {
	if err := s.ready(userID); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrMobileTaskNotFound
	}
	deletedAt := s.now().UTC()
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_tasks SET deleted_at = ?, updated_at = ? WHERE user_id = ? AND id = ? AND deleted_at IS NULL`),
		deletedAt, deletedAt, userID, id)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrMobileTaskNotFound
	}
	return &MobileTaskDeleteResult{ID: id, Deleted: true, DeletedAt: deletedAt}, nil
}

// Transition persists executor-owned status changes while preserving the
// protocol's terminal-state rules. Repeating the same terminal state is safe.
func (s *MobileTaskService) Transition(ctx context.Context, userID int64, id string, input MobileTaskTransitionInput) (*MobileTask, error) {
	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if IsTerminalMobileTaskStatus(current.Status) && current.Status == input.Status {
		return current, nil
	}
	if input.Progress != nil && *input.Progress < current.Progress {
		return nil, ErrMobileTaskInvalidProgress
	}
	updated := *current
	if input.Progress != nil {
		updated.Progress = *input.Progress
	}
	if input.Resource != nil {
		updated.Resource = sanitizeMobileTaskResource(input.Resource)
	}
	if input.Artifacts != nil {
		updated.Artifacts = sanitizeMobileTaskArtifacts(input.Artifacts)
	}
	updated.Error = sanitizeMobileTaskError(input.Error)
	if current.Status == input.Status && !IsTerminalMobileTaskStatus(current.Status) {
		if input.Progress == nil && input.Resource == nil && input.Artifacts == nil && input.Error == nil {
			return current, nil
		}
		if err := ValidateMobileTask(updated); err != nil {
			return nil, err
		}
		updated.Cancellable = CanCancelMobileTask(updated)
		updated.Retryable = CanRetryMobileTask(updated)
		return s.persistTransition(ctx, userID, *current, updated)
	}
	updated, err = TransitionMobileTask(updated, input.Status, s.now())
	if err != nil {
		return nil, err
	}
	return s.persistTransition(ctx, userID, *current, updated)
}

func (s *MobileTaskService) persistTransition(ctx context.Context, userID int64, before, after MobileTask) (*MobileTask, error) {
	resourceJSON, artifactsJSON, errorJSON, err := encodeMobileTaskJSON(after)
	if err != nil {
		return nil, err
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_tasks SET status = ?, progress = ?, resource = ?, artifacts = ?, error = ?, started_at = ?, finished_at = ?, updated_at = ? WHERE user_id = ? AND id = ? AND status = ? AND progress <= ?`),
		string(after.Status), after.Progress, nullableJSON(resourceJSON), nullableJSON(artifactsJSON), nullableJSON(errorJSON), nullableTime(after.StartedAt), nullableTime(after.FinishedAt), s.now().UTC(), userID, after.ID, string(before.Status), after.Progress)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		latest, getErr := s.Get(ctx, userID, after.ID)
		if getErr != nil {
			return nil, getErr
		}
		if latest.Status == after.Status && IsTerminalMobileTaskStatus(after.Status) {
			return latest, nil
		}
		if latest.Progress > after.Progress {
			return nil, ErrMobileTaskInvalidProgress
		}
		return nil, ErrMobileTaskInvalidTransition
	}
	return s.Get(ctx, userID, after.ID)
}

func (s *MobileTaskService) insert(ctx context.Context, userID int64, task MobileTask) error {
	resourceJSON, artifactsJSON, errorJSON, err := encodeMobileTaskJSON(task)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, s.bind(`INSERT INTO mobile_tasks (id, user_id, kind, operation, status, progress, parent_task_id, retry_of, client_request_id, resource, artifacts, error, protocol_version, created_at, updated_at, started_at, finished_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		task.ID, userID, string(task.Kind), task.Operation, string(task.Status), task.Progress,
		nullableString(task.ParentTaskID), nullableString(task.RetryOf), task.ClientRequestID,
		nullableJSON(resourceJSON), nullableJSON(artifactsJSON), nullableJSON(errorJSON), task.Version, task.CreatedAt, task.CreatedAt,
		nullableTime(task.StartedAt), nullableTime(task.FinishedAt))
	return err
}

func (s *MobileTaskService) getByClientRequestID(ctx context.Context, userID int64, requestID string) (*MobileTask, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, ErrMobileTaskNotFound
	}
	return s.queryOne(ctx, mobileTaskSelect+" WHERE user_id = ? AND client_request_id = ?", userID, strings.TrimSpace(requestID))
}

func (s *MobileTaskService) queryOne(ctx context.Context, query string, args ...any) (*MobileTask, error) {
	task, err := scanMobileTask(s.db.QueryRowContext(ctx, s.bind(query), args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMobileTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *MobileTaskService) ready(userID int64) error {
	if s == nil || s.db == nil {
		return ErrMobileTaskStoreUnavailable
	}
	if userID <= 0 {
		return ErrMobileTaskNotFound
	}
	return nil
}

func (s *MobileTaskService) bind(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var builder strings.Builder
	position := 1
	for _, char := range query {
		if char == '?' {
			fmt.Fprintf(&builder, "$%d", position)
			position++
		} else {
			_, _ = builder.WriteRune(char)
		}
	}
	return builder.String()
}

const mobileTaskSelect = `SELECT id, kind, operation, status, progress, parent_task_id, retry_of, client_request_id, resource, artifacts, error, protocol_version, created_at, started_at, finished_at FROM mobile_tasks`

type mobileTaskScanner interface {
	Scan(dest ...any) error
}

func scanMobileTask(scanner mobileTaskScanner) (MobileTask, error) {
	var task MobileTask
	var kind, status string
	var parentTaskID, retryOf sql.NullString
	var resourceJSON, artifactsJSON, errorJSON []byte
	var startedAt, finishedAt sql.NullTime
	err := scanner.Scan(&task.ID, &kind, &task.Operation, &status, &task.Progress, &parentTaskID, &retryOf,
		&task.ClientRequestID, &resourceJSON, &artifactsJSON, &errorJSON, &task.Version, &task.CreatedAt, &startedAt, &finishedAt)
	if err != nil {
		return MobileTask{}, err
	}
	task.Kind = MobileTaskKind(kind)
	task.Status = MobileTaskStatus(status)
	task.ParentTaskID = parentTaskID.String
	task.RetryOf = retryOf.String
	if startedAt.Valid {
		value := startedAt.Time.UTC()
		task.StartedAt = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time.UTC()
		task.FinishedAt = &value
	}
	if len(resourceJSON) > 0 {
		var resource MobileTaskResource
		if err := json.Unmarshal(resourceJSON, &resource); err != nil {
			return MobileTask{}, err
		}
		task.Resource = sanitizeMobileTaskResource(&resource)
	}
	task.Artifacts = make([]MobileTaskArtifact, 0)
	if len(artifactsJSON) > 0 {
		if err := json.Unmarshal(artifactsJSON, &task.Artifacts); err != nil {
			return MobileTask{}, err
		}
		task.Artifacts = sanitizeMobileTaskArtifacts(task.Artifacts)
	}
	if len(errorJSON) > 0 {
		var taskError MobileTaskError
		if err := json.Unmarshal(errorJSON, &taskError); err != nil {
			return MobileTask{}, err
		}
		task.Error = sanitizeMobileTaskError(&taskError)
	}
	task.CreatedAt = task.CreatedAt.UTC()
	task.Cancellable = CanCancelMobileTask(task)
	task.Retryable = CanRetryMobileTask(task)
	return task, ValidateMobileTask(task)
}

func encodeMobileTaskJSON(task MobileTask) ([]byte, []byte, []byte, error) {
	var resourceJSON, errorJSON []byte
	var err error
	if task.Resource != nil {
		resourceJSON, err = json.Marshal(sanitizeMobileTaskResource(task.Resource))
		if err != nil {
			return nil, nil, nil, err
		}
	}
	artifactsJSON, err := json.Marshal(sanitizeMobileTaskArtifacts(task.Artifacts))
	if err != nil {
		return nil, nil, nil, err
	}
	if task.Error != nil {
		errorJSON, err = json.Marshal(sanitizeMobileTaskError(task.Error))
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return resourceJSON, artifactsJSON, errorJSON, nil
}

func sanitizeMobileTaskResource(input *MobileTaskResource) *MobileTaskResource {
	if input == nil {
		return nil
	}
	return &MobileTaskResource{Type: boundedMobileTaskText(input.Type), ID: boundedMobileTaskText(input.ID)}
}

func sanitizeMobileTaskError(input *MobileTaskError) *MobileTaskError {
	if input == nil {
		return nil
	}
	return &MobileTaskError{
		Code:      boundedMobileTaskText(input.Code),
		Message:   boundedMobileTaskText(logredact.RedactText(input.Message, mobileTaskSensitiveKeys...)),
		Retryable: input.Retryable,
		Details:   logredact.RedactMap(input.Details, mobileTaskSensitiveKeys...),
	}
}

func sanitizeMobileTaskArtifacts(input []MobileTaskArtifact) []MobileTaskArtifact {
	if len(input) == 0 {
		return []MobileTaskArtifact{}
	}
	if len(input) > mobileTaskMaxArtifacts {
		input = input[:mobileTaskMaxArtifacts]
	}
	output := make([]MobileTaskArtifact, 0, len(input))
	for _, artifact := range input {
		artifact.ID = boundedMobileTaskText(artifact.ID)
		artifact.Kind = boundedMobileTaskText(artifact.Kind)
		artifact.Name = boundedMobileTaskText(logredact.RedactText(artifact.Name, mobileTaskSensitiveKeys...))
		artifact.ContentType = boundedMobileTaskText(artifact.ContentType)
		artifact.URL = sanitizeMobileTaskArtifactURL(artifact.URL)
		artifact.Metadata = logredact.RedactMap(artifact.Metadata, mobileTaskSensitiveKeys...)
		output = append(output, artifact)
	}
	return output
}

func sanitizeMobileTaskArtifactURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return boundedMobileTaskText(parsed.String())
}

func boundedMobileTaskText(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= mobileTaskMaxTextRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:mobileTaskMaxTextRunes])
}

func normalizeMobileTaskListFilter(filter MobileTaskListFilter) MobileTaskListFilter {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = mobileTaskDefaultPageSize
	}
	if filter.PageSize > mobileTaskMaxPageSize {
		filter.PageSize = mobileTaskMaxPageSize
	}
	return filter
}

func sameMobileTaskCreate(existing *MobileTask, input MobileTaskCreateInput) bool {
	if existing == nil || existing.Kind != input.Kind || existing.Operation != input.Operation || existing.ParentTaskID != input.ParentTaskID {
		return false
	}
	existingResource := sanitizeMobileTaskResource(existing.Resource)
	inputResource := sanitizeMobileTaskResource(input.Resource)
	if existingResource == nil || inputResource == nil {
		return existingResource == nil && inputResource == nil
	}
	return existingResource.Type == inputResource.Type && existingResource.ID == inputResource.ID
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}
