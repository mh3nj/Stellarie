package privacy

import (
	"testing"
	"time"
)

var retBase = time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

func TestComputeDeleteAfter_Never(t *testing.T) {
	if got := ComputeDeleteAfter(retBase, RetentionNever); got != nil {
		t.Fatalf("got %v, want nil for RetentionNever", got)
	}
}

func TestComputeDeleteAfter_SevenDays(t *testing.T) {
	got := ComputeDeleteAfter(retBase, RetentionSevenDays)
	if got == nil {
		t.Fatal("expected a non-nil delete-after time")
	}
	want := retBase.AddDate(0, 0, 7)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestShouldPurge_NilNeverPurges(t *testing.T) {
	s := Screenshot{DeleteAfter: nil}
	if ShouldPurge(s, retBase.AddDate(10, 0, 0)) {
		t.Fatal("a nil DeleteAfter must never be purged, no matter how far in the future 'now' is")
	}
}

func TestShouldPurge_BeforeDeleteAfter(t *testing.T) {
	deleteAfter := retBase.AddDate(0, 0, 7)
	s := Screenshot{DeleteAfter: &deleteAfter}
	if ShouldPurge(s, retBase.AddDate(0, 0, 3)) {
		t.Fatal("must not purge before the delete-after time arrives")
	}
}

func TestShouldPurge_ExactlyAtDeleteAfter(t *testing.T) {
	deleteAfter := retBase.AddDate(0, 0, 7)
	s := Screenshot{DeleteAfter: &deleteAfter}
	if !ShouldPurge(s, deleteAfter) {
		t.Fatal("must purge exactly at the delete-after instant, not only strictly after it")
	}
}

func TestShouldPurge_LongAfterDeleteAfter(t *testing.T) {
	deleteAfter := retBase.AddDate(0, 0, 7)
	s := Screenshot{DeleteAfter: &deleteAfter}
	if !ShouldPurge(s, retBase.AddDate(1, 0, 0)) {
		t.Fatal("must purge well after the delete-after time")
	}
}
