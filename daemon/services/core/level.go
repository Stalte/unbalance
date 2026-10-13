package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/teris-io/shortid"

	"unbalance/daemon/algorithm"
	"unbalance/daemon/common"
	"unbalance/daemon/domain"
	"unbalance/daemon/lib"
	"unbalance/daemon/logger"
)

// LEVEL PLANNER
func (c *Core) levelPlanPrepare(setup domain.LevelSetup) {
	now := time.Now()

	if c.state.Status != common.OpNeutral {
		logger.Yellow("unbalanced is busy: %d", c.state.Status)
		return
	}

	unraid := c.refreshUnraid()

	// fail fast on an invalid selection, before going busy
	paths := make([]string, 0, len(setup.Disks))
	for _, name := range setup.Disks {
		path := filepath.Join("/", "mnt", name)
		if !slices.ContainsFunc(unraid.Disks, func(disk *domain.Disk) bool { return disk.Path == path }) {
			logger.Yellow("levelPlan:invalid disk (%s)", name)
			c.publishOperationError("unable to plan: disk %q is not part of the array", name)
			return
		}
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}

	if len(paths) < 2 {
		c.publishOperationError("unable to plan: choose at least two disks to level")
		return
	}

	if len(setup.Selected) == 0 {
		c.publishOperationError("unable to plan: choose at least one folder to level")
		return
	}

	for _, folder := range setup.Selected {
		found := false
		for _, path := range paths {
			if _, err := os.Stat(filepath.Join(path, folder)); err == nil {
				found = true
				break
			}
		}
		if !found {
			logger.Yellow("levelPlan:invalid selection (%s): not present on any chosen disk", folder)
			c.publishOperationError("unable to plan: %q was not found on any of the chosen disks", folder)
			return
		}
	}

	c.state.Status = common.OpLevelPlan
	c.state.Unraid = unraid

	// make sure a previous stop request doesn't cancel this plan
	c.stopped.Store(false)
	ctx := c.newPlanContext()

	plan := &domain.Plan{
		Started:       now,
		ChosenFolders: setup.Selected,
		VDisks:        make(map[string]*domain.VDisk),
	}

	for _, disk := range c.state.Unraid.Disks {
		plan.VDisks[disk.Path] = &domain.VDisk{
			Path:        disk.Path,
			CurrentFree: disk.Free,
			PlannedFree: disk.Free,
			Bin:         nil,
			// Dst marks the disks chosen for levelling; Src is set later on
			// the disks the plan takes data from
			Dst: slices.Contains(paths, disk.Path),
		}
	}

	go c.levelPlan(ctx, plan)
}

func (c *Core) levelPlan(ctx context.Context, plan *domain.Plan) {
	logger.Blue("Running level planner ...")

	packet := &domain.Packet{Topic: common.EventLevelPlanStarted, Payload: "Planning started"}
	c.ctx.Hub.Pub(packet, "socket:broadcast")

	c.printDisks(c.state.Unraid.Disks, c.state.Unraid.BlockSize)

	disks := make([]*domain.Disk, 0)
	for _, disk := range c.state.Unraid.Disks {
		if plan.VDisks[disk.Path].Dst {
			disks = append(disks, disk)
		}
	}

	items, ownerIssue, groupIssue, folderIssue, fileIssue := c.getItemsAndIssues(ctx, c.state.Status, c.state.Unraid.BlockSize, reItems, reStat, disks, plan.ChosenFolders)

	if c.stopped.Load() {
		c.planCancelled(common.EventLevelPlanCancelled)
		return
	}

	plan.OwnerIssue = ownerIssue
	plan.GroupIssue = groupIssue
	plan.FolderIssue = folderIssue
	plan.FileIssue = fileIssue

	logger.Blue("levelPlan:items(%d)", len(items))

	c.publishFoundItems(common.EventLevelPlanProgress, items)

	logger.Blue("levelPlan:issues:owner(%d),group(%d),folder(%d),file(%d)", plan.OwnerIssue, plan.GroupIssue, plan.FolderIssue, plan.FileIssue)

	c.levelAllocate(plan, disks, items)

	c.endPlan(c.state.Status, plan, c.state.Unraid.Disks, items, make([]*domain.Item, 0))
	c.levelPlanEnd(plan)
}

// levelAllocate runs the level algorithm over the chosen disks and records
// the resulting moves in the plan: each destination disk gets a bin with the
// items it will receive, and every source disk is flagged as Src
func (c *Core) levelAllocate(plan *domain.Plan, disks []*domain.Disk, items []*domain.Item) {
	packet := &domain.Packet{Topic: common.EventLevelPlanProgress, Payload: "Working out the moves ..."}
	c.ctx.Hub.Pub(packet, "socket:broadcast")

	levelDisks := make([]algorithm.LevelDisk, 0, len(disks))
	for _, disk := range disks {
		levelDisks = append(levelDisks, algorithm.LevelDisk{
			Path:     disk.Path,
			Size:     disk.Size,
			Free:     disk.Free,
			Reserved: lib.Max(common.ReservedSpace, c.getReservedAmount(disk.Size)),
		})
	}

	// an empty chosen folder shows up as an item for the folder itself; it
	// has nothing to level, and moving it would move the whole share
	movable := make([]*domain.Item, 0, len(items))
	for _, item := range items {
		if !slices.Contains(plan.ChosenFolders, item.Path) {
			movable = append(movable, item)
		}
	}

	result := algorithm.Level(levelDisks, movable, algorithm.LevelTolerance)

	plan.BytesToTransfer = 0
	for _, move := range result.Moves {
		dst := plan.VDisks[move.Dst]
		if dst.Bin == nil {
			dst.Bin = &domain.Bin{}
		}
		dst.Bin.Add(move.Item)
		dst.PlannedFree -= move.Item.Size

		src := plan.VDisks[move.Item.Location]
		src.Src = true
		src.PlannedFree += move.Item.Size

		plan.BytesToTransfer += move.Item.Size
	}

	// only the disks that receive data stay flagged as destinations
	for _, disk := range disks {
		vdisk := plan.VDisks[disk.Path]
		vdisk.Dst = vdisk.Bin != nil
	}

	lines := []string{fmt.Sprintf("Target fill: %.1f%% on each of the %d chosen disks", result.Target*100, len(disks))}
	if len(result.Split) > 0 {
		lines = append(lines, fmt.Sprintf("%d folder(s) are currently spread over more than one disk; if moved, they are moved whole onto one disk:", len(result.Split)))
		lines = append(lines, result.Split...)
	}
	for _, disk := range disks {
		vdisk := plan.VDisks[disk.Path]
		lines = append(lines, fmt.Sprintf("%s: %s -> %s", disk.Name, fillPercent(disk.Size, vdisk.CurrentFree), fillPercent(disk.Size, vdisk.PlannedFree)))
	}
	lines = append(lines, fmt.Sprintf("%d folder(s)/file(s) to move, %s in total", len(result.Moves), lib.ByteSize(plan.BytesToTransfer)))

	logger.Blue("levelPlan:%s", strings.Join(lines, "\nlevelPlan:"))

	packet = &domain.Packet{Topic: common.EventLevelPlanProgress, Payload: strings.Join(lines, "\n")}
	c.ctx.Hub.Pub(packet, "socket:broadcast")
}

func fillPercent(size, free uint64) string {
	if size == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", float64(size-free)/float64(size)*100)
}

func (c *Core) levelPlanEnd(plan *domain.Plan) {
	c.state.Status = common.OpNeutral
	c.clearPlanContext()

	planView, err := c.storePendingPlan(planFlowLevel, plan)
	if err != nil {
		logger.Red("unable to store level plan: %s", err)
		c.publishOperationError("unable to store level plan: %s", err)
		return
	}

	packet := &domain.Packet{Topic: common.EventLevelPlanEnded, Payload: planView}
	c.ctx.Hub.Pub(packet, "socket:broadcast")
}

// LEVEL TRANSFER
func (c *Core) levelMove(planID string) {
	plan, err := c.takePendingPlan(planID, planFlowLevel)
	if err != nil {
		logger.Yellow("levelMove: %s", err)
		c.publishOperationError("unable to start level move: %s", err)
		return
	}

	c.state.Status = common.OpLevelMove
	c.state.Unraid = c.refreshUnraid()
	c.state.Operation = c.createLevelOperation(plan)

	go c.runOperation("Move")
}

func (c *Core) createLevelOperation(plan domain.Plan) *domain.Operation {
	operation := &domain.Operation{
		ID:              shortid.MustGenerate(),
		OpKind:          common.OpLevelMove,
		BytesToTransfer: plan.BytesToTransfer,
		DryRun:          c.ctx.DryRun,
	}

	operation.RsyncArgs = append([]string{common.RsyncArgs}, c.ctx.RsyncArgs...)

	// user may have changed dry-run setting, adjust for it
	if operation.DryRun {
		operation.RsyncArgs = append(operation.RsyncArgs, "--dry-run")
	}
	operation.RsyncStrArgs = strings.Join(operation.RsyncArgs, " ")

	operation.Commands = make([]*domain.Command, 0)

	// commands are grouped by destination disk, in array order
	for _, disk := range c.state.Unraid.Disks {
		vdisk := plan.VDisks[disk.Path]
		if vdisk == nil || vdisk.Bin == nil {
			continue
		}

		for _, item := range vdisk.Bin.Items {
			entry := strings.TrimPrefix(item.Path, string(filepath.Separator))

			operation.Commands = append(operation.Commands, &domain.Command{
				ID:     shortid.MustGenerate(),
				Src:    item.Location,
				Dst:    vdisk.Path + string(filepath.Separator),
				Entry:  entry,
				Size:   item.Size,
				Status: common.CmdPending,
			})
		}
	}

	return operation
}
