package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestCrossedMilestones_NoCrossings(t *testing.T) {
	got := crossedMilestones(100, 500)
	if len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
}

func TestCrossedMilestones_FirstTier(t *testing.T) {
	got := crossedMilestones(900, 1500)
	if !reflect.DeepEqual(got, []string{"first_1k"}) {
		t.Errorf("got %v, want [first_1k]", got)
	}
}

func TestCrossedMilestones_SkipAlreadyCrossedLowerTier(t *testing.T) {
	got := crossedMilestones(5_000, 12_000)
	if !reflect.DeepEqual(got, []string{"first_10k"}) {
		t.Errorf("got %v, want [first_10k]", got)
	}
}

func TestCrossedMilestones_SingleRequestCrossesMultipleTiers(t *testing.T) {
	got := crossedMilestones(0, 200_000)
	want := []string{"first_1k", "first_10k", "first_100k"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCrossedMilestones_ExactBoundaryCrosses(t *testing.T) {
	// (prev, new] semantics: hitting threshold exactly counts.
	got := crossedMilestones(999, 1000)
	if !reflect.DeepEqual(got, []string{"first_1k"}) {
		t.Errorf("got %v, want [first_1k]", got)
	}
}

func TestCrossedMilestones_AllTiersInOneShot(t *testing.T) {
	got := crossedMilestones(0, 2_000_000)
	want := []string{"first_1k", "first_10k", "first_100k", "first_1m"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMilestoneThresholds_SortedAndComplete(t *testing.T) {
	got := MilestoneThresholds()
	want := []int64{1_000, 10_000, 100_000, 1_000_000}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCheckAndPublishUsageMilestone_NoOpWhenRedisDisabled(t *testing.T) {
	prevEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prevEnabled })

	// Must not panic; no publisher calls expected (redis disabled returns early).
	CheckAndPublishUsageMilestone(context.Background(), 1, 1500)
}

func TestCheckAndPublishUsageMilestone_NoOpForZeroOrNegative(t *testing.T) {
	// Must short-circuit before touching Redis or publisher.
	CheckAndPublishUsageMilestone(context.Background(), 0, 100)
	CheckAndPublishUsageMilestone(context.Background(), -1, 100)
	CheckAndPublishUsageMilestone(context.Background(), 1, 0)
	CheckAndPublishUsageMilestone(context.Background(), 1, -5)
}
