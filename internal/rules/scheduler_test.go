package rules

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"
	"time"
)

type fakeSchedulerClock struct {
	mu   sync.Mutex
	now  time.Time
	wake chan struct{}
}

func newFakeSchedulerClock(now time.Time) *fakeSchedulerClock {
	return &fakeSchedulerClock{now: now, wake: make(chan struct{})}
}
func (c *fakeSchedulerClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeSchedulerClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	close(c.wake)
	c.wake = make(chan struct{})
	c.mu.Unlock()
}
func (c *fakeSchedulerClock) Wait(ctx context.Context, d time.Duration) error {
	return c.WaitUntil(ctx, c.Now().Add(d))
}
func (c *fakeSchedulerClock) WaitUntil(ctx context.Context, at time.Time) error {
	for {
		c.mu.Lock()
		now, wake := c.now, c.wake
		c.mu.Unlock()
		if !now.Before(at) {
			return nil
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-wake:
		}
	}
}

func TestSchedulerRebuildsFromPersistentNextUpdate(t *testing.T) {
	clock := newFakeSchedulerClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	service, _ := newRuleService(t, nil)
	service.clock = clock.Now
	_, err := service.Create(context.Background(), CreateInput{Name: "scheduled", URL: "https://example.test/a", Format: FormatBinary, UpdateInterval: "1h"})
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.store.list()
	if err != nil {
		t.Fatal(err)
	}
	next := clock.Now().Add(time.Minute)
	items[0].NextUpdateAt = &next
	if err := service.store.write(items); err != nil {
		t.Fatal(err)
	}
	service.SetDownloader(failingRuleDownloader{fmt.Errorf("offline")})
	scheduler := NewScheduler(service, clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- scheduler.Run(ctx) }()
	clock.Advance(time.Minute)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		responses, _ := service.List(context.Background())
		if responses[0].LastAttemptAt != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("scheduler did not return cancellation")
	}
	responses, _ := service.List(context.Background())
	if responses[0].LastAttemptAt == nil || responses[0].NextUpdateAt == nil || !responses[0].NextUpdateAt.Equal(clock.Now().Add(time.Hour)) {
		t.Fatalf("scheduled state=%+v", responses[0])
	}
}

type recordingResourceLocker struct {
	mu  sync.Mutex
	ids []string
}

func (l *recordingResourceLocker) Lock(_ context.Context, id string) (func(), error) {
	l.mu.Lock()
	l.ids = append(l.ids, id)
	l.mu.Unlock()
	return func() {}, nil
}
func (l *recordingResourceLocker) AcquiredIDs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.ids...)
}

func TestSnapshotLocksResourcesInIDOrder(t *testing.T) {
	service, layout := newRuleService(t, nil)
	service.random = bytes.NewReader(append(bytes.Repeat([]byte{0x11}, 16), bytes.Repeat([]byte{0x22}, 16)...))
	a, _ := service.Create(context.Background(), CreateInput{Name: "a", URL: "https://example.test/a", Format: FormatBinary})
	b, _ := service.Create(context.Background(), CreateInput{Name: "b", URL: "https://example.test/b", Format: FormatBinary})
	items, _ := service.store.list()
	for i := range items {
		payload := []byte(items[i].ID)
		if err := os.MkdirAll(layout.RuleSetsDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(items[i].LocalPath, payload, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(payload)
		items[i].LastSuccessAt = ptrTime(time.Now())
		items[i].LastDigest = fmt.Sprintf("%x", sum)
	}
	if err := service.store.write(items); err != nil {
		t.Fatal(err)
	}
	locker := &recordingResourceLocker{}
	service.SetResourceLocker(locker)
	_, release, err := service.SnapshotForBindings(context.Background(), []RuleBinding{{ResourceID: b.ID, Enabled: true}, {ResourceID: a.ID, Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	expected := []string{a.ID, b.ID}
	slices.Sort(expected)
	if !slices.Equal(locker.AcquiredIDs(), expected) {
		t.Fatalf("order=%v expected=%v", locker.AcquiredIDs(), expected)
	}
}

func TestSnapshotRejectsResourceFileDrift(t *testing.T) {
	service, layout := newRuleService(t, nil)
	resource, _ := service.Create(context.Background(), CreateInput{Name: "a", URL: "https://example.test/a", Format: FormatBinary})
	path, _, _ := layout.RuleSetPath(resource.ID, ".srs")
	if err := os.MkdirAll(layout.RuleSetsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	items, _ := service.store.list()
	items[0].LastSuccessAt = ptrTime(time.Now())
	items[0].LastDigest = fmt.Sprintf("%x", sha256.Sum256([]byte("old")))
	if err := service.store.write(items); err != nil {
		t.Fatal(err)
	}
	_, release, err := service.SnapshotForBindings(context.Background(), []RuleBinding{{ResourceID: resource.ID, Enabled: true}})
	release()
	if err != ErrResourceFileDrift {
		t.Fatalf("err=%v", err)
	}
}

func TestConcurrentSnapshotsWithReverseBindingsDoNotDeadlock(t *testing.T) {
	service, layout := newRuleService(t, nil)
	service.random = bytes.NewReader(append(bytes.Repeat([]byte{0x11}, 16), bytes.Repeat([]byte{0x22}, 16)...))
	a, err := service.Create(context.Background(), CreateInput{Name: "a", URL: "https://example.test/a", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	b, err := service.Create(context.Background(), CreateInput{Name: "b", URL: "https://example.test/b", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	items, _ := service.store.list()
	if err := os.MkdirAll(layout.RuleSetsDir, 0700); err != nil {
		t.Fatal(err)
	}
	for index := range items {
		payload := []byte(items[index].ID)
		if err := os.WriteFile(items[index].LocalPath, payload, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(payload)
		items[index].LastSuccessAt = ptrTime(time.Now())
		items[index].LastDigest = fmt.Sprintf("%x", sum)
	}
	if err := service.store.write(items); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	for _, bindings := range [][]RuleBinding{{{ResourceID: a.ID, Enabled: true}, {ResourceID: b.ID, Enabled: true}}, {{ResourceID: b.ID, Enabled: true}, {ResourceID: a.ID, Enabled: true}}} {
		go func(bindings []RuleBinding) {
			_, release, err := service.SnapshotForBindings(context.Background(), bindings)
			if err == nil {
				release()
			}
			done <- err
		}(bindings)
	}
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("snapshot lock order deadlocked")
		}
	}
}
