package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cskr/pubsub"

	"unbalance/daemon/common"
	"unbalance/daemon/domain"
)

func TestLevelPlanKeepsShowsTogetherAndBuildsCommands(t *testing.T) {
	full := t.TempDir()
	empty := t.TempDir()

	// two shows with seasons on the full disk; each show is one unit
	for _, show := range []string{"Billions", "Severance"} {
		for _, season := range []string{"S01", "S02"} {
			dir := filepath.Join(full, "tv", show, season)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "e01.mkv"), make([]byte, 4096), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Join(empty, "tv"), 0o755); err != nil {
		t.Fatal(err)
	}

	disks := []*domain.Disk{
		{Name: "disk1", Path: full, Size: 20000, Free: 4000},
		{Name: "disk2", Path: empty, Size: 20000, Free: 20000},
	}

	c := &Core{
		ctx:          &domain.Context{Hub: pubsub.New(common.ChanCapacity)},
		state:        &domain.State{Status: common.OpLevelPlan, Unraid: &domain.Unraid{Disks: disks}},
		pendingPlans: make(map[string]*planTicket),
	}

	plan := &domain.Plan{ChosenFolders: []string{"tv"}, VDisks: make(map[string]*domain.VDisk)}
	for _, disk := range disks {
		plan.VDisks[disk.Path] = &domain.VDisk{Path: disk.Path, CurrentFree: disk.Free, PlannedFree: disk.Free, Dst: true}
	}

	items, _, _, _, _ := c.getItemsAndIssues(context.Background(), common.OpLevelPlan, 0, reItems, reStat, disks, plan.ChosenFolders)
	// one item per show, plus the empty tv folder on disk2
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// the 1GB floor on reserved space can't be lowered, so allocate against
	// disks big enough to clear it
	for _, disk := range disks {
		disk.Size *= 1 << 20
		disk.Free *= 1 << 20
		plan.VDisks[disk.Path].CurrentFree = disk.Free
		plan.VDisks[disk.Path].PlannedFree = disk.Free
	}
	for _, item := range items {
		item.Size *= 1 << 20
	}

	c.levelAllocate(plan, disks, items)

	if !plan.VDisks[full].Src || plan.VDisks[full].Dst {
		t.Fatalf("expected disk1 to be the source only: %+v", plan.VDisks[full])
	}
	bin := plan.VDisks[empty].Bin
	if bin == nil || len(bin.Items) != 1 {
		t.Fatalf("expected exactly one show to move to disk2, got %+v", bin)
	}

	c.state.Unraid.Disks = disks
	operation := c.createLevelOperation(*plan)
	if operation.OpKind != common.OpLevelMove || len(operation.Commands) != 1 {
		t.Fatalf("unexpected operation: %+v", operation)
	}

	cmd := operation.Commands[0]
	if cmd.Src != full || cmd.Dst != empty+"/" {
		t.Fatalf("unexpected command roots: src %s dst %s", cmd.Src, cmd.Dst)
	}
	if cmd.Entry != "tv/Billions" && cmd.Entry != "tv/Severance" {
		t.Fatalf("expected a whole show to move, got %s", cmd.Entry)
	}
	if !isMove(operation.OpKind) || !prunesParents(operation.OpKind) {
		t.Fatalf("level move must remove and prune its sources")
	}
}
