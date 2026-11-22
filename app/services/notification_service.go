package services

import (
	"context"
	"time"

	"stellarei/internal/notify"
)

// NotificationView is a notification history record shaped for the
// frontend.
type NotificationView struct {
	ID          int64  `json:"id"`
	Source      string `json:"source"`
	SourceID    int64  `json:"sourceId,omitempty"`
	Title       string `json:"title"`
	Body        string `json:"body,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	DismissedAt int64  `json:"dismissedAt,omitempty"`
}

// NotificationService is bound to the frontend via application.NewService
// in main.go. Read/dismiss only - notifications are created exclusively
// by internal/scheduler (via internal/notify.Notifier), never directly
// by the frontend, so there's no Create method here.
type NotificationService struct {
	repo *notify.Repository
}

func NewNotificationService(repo *notify.Repository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) ListRecent(limit int) ([]NotificationView, error) {
	rows, err := s.repo.ListRecent(context.Background(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]NotificationView, 0, len(rows))
	for _, r := range rows {
		v := NotificationView{
			ID: r.ID, Source: string(r.Source), SourceID: r.SourceID,
			Title: r.Title, Body: r.Body, CreatedAt: r.CreatedAt.Unix(),
		}
		if r.DismissedAt != nil {
			v.DismissedAt = r.DismissedAt.Unix()
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *NotificationService) Dismiss(id int64) error {
	return s.repo.Dismiss(context.Background(), id, time.Now())
}
