package services

import (
	"context"

	"stellarei/internal/sessions"
)

// SessionView is what the frontend Timeline renders - a flattened,
// JSON-friendly copy of sessions.View. Timestamps are Unix seconds
// (matching how the frontend already handles times elsewhere) rather
// than RFC3339 strings, since Timeline needs to do a lot of numeric
// range/zoom math with them.
type SessionView struct {
	ID              int64   `json:"id"`
	StartedAt       int64   `json:"startedAt"`
	EndedAt         int64   `json:"endedAt"`
	ApplicationID   int64   `json:"applicationId"`
	ApplicationName string  `json:"applicationName"`
	ProjectID       int64   `json:"projectId"`
	ProjectName     string  `json:"projectName"`
	ProjectColor    string  `json:"projectColor"`
	CategoryID      int64   `json:"categoryId"`
	CategoryName    string  `json:"categoryName"`
	CategoryColor   string  `json:"categoryColor"`
	Confidence      float64 `json:"confidence"`
	ManuallyEdited  bool    `json:"manuallyEdited"`
	Excluded        bool    `json:"excluded"`
	Note            string  `json:"note"`
}

// SessionService is bound to the frontend for the Timeline (spec section
// 13): listing sessions for whatever range is currently zoomed to, and
// applying manual corrections (section 63) - reassign project, exclude,
// annotate, split, merge.
type SessionService struct {
	repo sessions.Repository
}

func NewSessionService(repo sessions.Repository) *SessionService {
	return &SessionService{repo: repo}
}

// ListForRange returns every session overlapping [startUnix, endUnix).
func (s *SessionService) ListForRange(startUnix, endUnix int64) ([]SessionView, error) {
	rows, err := s.repo.ListForRange(context.Background(), startUnix, endUnix)
	if err != nil {
		return nil, err
	}
	out := make([]SessionView, 0, len(rows))
	for _, v := range rows {
		out = append(out, SessionView{
			ID:              v.ID,
			StartedAt:       v.StartedAt,
			EndedAt:         v.EndedAt,
			ApplicationID:   v.ApplicationID,
			ApplicationName: v.ApplicationName,
			ProjectID:       v.ProjectID,
			ProjectName:     v.ProjectName,
			ProjectColor:    v.ProjectColor,
			CategoryID:      v.CategoryID,
			CategoryName:    v.CategoryName,
			CategoryColor:   v.CategoryColor,
			Confidence:      v.Confidence,
			ManuallyEdited:  v.ManuallyEdited,
			Excluded:        v.Excluded,
			Note:            v.Note,
		})
	}
	return out, nil
}

// UpdateCorrection reassigns a session's project (0 clears it) and
// category (0 clears it), sets its excluded flag and note, and marks it
// manually_edited (section 63: corrections take precedence over
// anything automated).
func (s *SessionService) UpdateCorrection(id int64, projectID, categoryID int64, excluded bool, note string) error {
	return s.repo.UpdateCorrection(context.Background(), id, projectID, categoryID, excluded, note)
}

// Split divides a session at splitAtUnix (must fall strictly within its
// time range) and returns the new session's id.
func (s *SessionService) Split(id int64, splitAtUnix int64) (int64, error) {
	return s.repo.Split(context.Background(), id, splitAtUnix)
}

// Merge combines mergeID into keepID - keepID's application/project/note
// survive, its time range extends to cover both, and mergeID is deleted.
func (s *SessionService) Merge(keepID, mergeID int64) error {
	return s.repo.Merge(context.Background(), keepID, mergeID)
}
