package rules

import (
	"context"
	"time"
)

type SchedulerClock interface {
	Now() time.Time
	Wait(context.Context, time.Duration) error
	WaitUntil(context.Context, time.Time) error
}

type systemClock struct{ now func() time.Time }

func (c systemClock) Now() time.Time { return c.now().UTC() }
func (c systemClock) Wait(ctx context.Context, duration time.Duration) error {
	return c.WaitUntil(ctx, c.Now().Add(duration))
}
func (c systemClock) WaitUntil(ctx context.Context, at time.Time) error {
	timer := time.NewTimer(time.Until(at))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}

type ScheduledUpdate struct {
	ResourceID string
	At         time.Time
}

type Scheduler struct {
	service *Service
	clock   SchedulerClock
}

func NewScheduler(service *Service, clock SchedulerClock) *Scheduler {
	if clock == nil {
		clock = systemClock{now: time.Now}
	}
	return &Scheduler{service: service, clock: clock}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.service.RebuildSchedule(ctx, s.clock.Now()); err != nil {
		return err
	}
	for {
		next, ok := s.service.NextScheduled()
		if !ok {
			if err := s.clock.Wait(ctx, time.Minute); err != nil {
				return context.Cause(ctx)
			}
			continue
		}
		if err := s.clock.WaitUntil(ctx, next.At); err != nil {
			return context.Cause(ctx)
		}
		_, err := s.service.UpdateNow(ctx, next.ResourceID)
		_ = s.service.RecordScheduleResult(context.Background(), next.ResourceID, err)
	}
}

func (s *Service) RebuildSchedule(ctx context.Context, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now = now.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return err
	}
	changed := false
	for index := range resources {
		resource := &resources[index]
		if effectiveSourceType(resource.SourceType) == SourceManual || resource.Disabled || resource.UpdateInterval == "" {
			if resource.NextUpdateAt != nil {
				resource.NextUpdateAt = nil
				changed = true
			}
			continue
		}
		if resource.NextUpdateAt == nil || !resource.NextUpdateAt.After(now) {
			at := now
			resource.NextUpdateAt = &at
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.store.write(resources)
}

func (s *Service) NextScheduled() (ScheduledUpdate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return ScheduledUpdate{}, false
	}
	var next ScheduledUpdate
	for _, resource := range resources {
		if effectiveSourceType(resource.SourceType) == SourceManual || resource.Disabled || resource.UpdateInterval == "" || resource.NextUpdateAt == nil {
			continue
		}
		if next.ResourceID == "" || resource.NextUpdateAt.Before(next.At) || (resource.NextUpdateAt.Equal(next.At) && resource.ID < next.ResourceID) {
			next = ScheduledUpdate{ResourceID: resource.ID, At: resource.NextUpdateAt.UTC()}
		}
	}
	return next, next.ResourceID != ""
}

func (s *Service) RecordScheduleResult(ctx context.Context, id string, updateErr error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := s.locks.Lock(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return err
	}
	index := indexOf(resources, id)
	if index < 0 {
		return ErrNotFound
	}
	resource := &resources[index]
	if effectiveSourceType(resource.SourceType) == SourceManual || resource.Disabled || resource.UpdateInterval == "" {
		resource.NextUpdateAt = nil
		return s.store.write(resources)
	}
	duration, err := time.ParseDuration(resource.UpdateInterval)
	if err != nil {
		return err
	}
	base := s.clock().UTC()
	if resource.NextUpdateAt != nil {
		base = resource.NextUpdateAt.UTC()
	}
	next := base.Add(duration)
	if !next.After(s.clock().UTC()) {
		next = s.clock().UTC().Add(duration)
	}
	resource.NextUpdateAt = &next
	return s.store.write(resources)
}
