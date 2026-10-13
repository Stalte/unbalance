package algorithm

import (
	"fmt"
	"math"
	"testing"

	"unbalance/daemon/domain"
)

const tb = uint64(1000 * 1000 * 1000 * 1000)
const gb = uint64(1000 * 1000 * 1000)

func fills(disks []LevelDisk, moves []LevelMove) map[string]float64 {
	used := map[string]float64{}
	for _, d := range disks {
		used[d.Path] = float64(d.Size - d.Free)
	}
	for _, m := range moves {
		used[m.Item.Location] -= float64(m.Item.Size)
		used[m.Dst] += float64(m.Item.Size)
	}
	out := map[string]float64{}
	for _, d := range disks {
		out[d.Path] = used[d.Path] / float64(d.Size)
	}
	return out
}

func spread(f map[string]float64) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range f {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	return hi - lo
}

func TestLevelEqualDisks(t *testing.T) {
	disks := []LevelDisk{
		{Path: "/mnt/disk1", Size: 10 * tb, Free: 2 * tb, Reserved: gb},
		{Path: "/mnt/disk2", Size: 10 * tb, Free: 10 * tb, Reserved: gb},
	}

	items := make([]*domain.Item, 0)
	for i := 0; i < 80; i++ {
		items = append(items, &domain.Item{Name: fmt.Sprintf("show%d", i), Path: fmt.Sprintf("tv/show%d", i), Location: "/mnt/disk1", Size: 100 * gb})
	}

	result := Level(disks, items, LevelTolerance)

	if math.Abs(result.Target-0.4) > 1e-9 {
		t.Fatalf("expected a 40%% target, got %f", result.Target)
	}
	if len(result.Moves) < 39 || len(result.Moves) > 40 {
		t.Fatalf("expected 39-40 moves, got %d", len(result.Moves))
	}
	if s := spread(fills(disks, result.Moves)); s > 0.02 {
		t.Fatalf("disks not levelled, spread %f", s)
	}
}

func TestLevelMixedSizesAimsAtEqualPercentage(t *testing.T) {
	disks := []LevelDisk{
		{Path: "/mnt/disk1", Size: 4 * tb, Free: 0},
		{Path: "/mnt/disk2", Size: 12 * tb, Free: 12 * tb},
	}

	items := make([]*domain.Item, 0)
	for i := 0; i < 400; i++ {
		items = append(items, &domain.Item{Path: fmt.Sprintf("movies/m%d", i), Location: "/mnt/disk1", Size: 10 * gb})
	}

	result := Level(disks, items, LevelTolerance)
	f := fills(disks, result.Moves)

	// 4TB used over 16TB = 25% on both disks
	for path, v := range f {
		if math.Abs(v-0.25) > 0.015 {
			t.Fatalf("%s planned at %.3f, expected ~0.25", path, v)
		}
	}
}

func TestLevelKeepsSplitFolderTogether(t *testing.T) {
	disks := []LevelDisk{
		{Path: "/mnt/disk1", Size: 10 * tb, Free: 2 * tb},
		{Path: "/mnt/disk2", Size: 10 * tb, Free: 4 * tb},
		{Path: "/mnt/disk3", Size: 10 * tb, Free: 10 * tb},
	}

	// a show already spread over disk1 and disk2, plus filler
	items := []*domain.Item{
		{Path: "tv/Billions", Location: "/mnt/disk1", Size: 1500 * gb},
		{Path: "tv/Billions", Location: "/mnt/disk2", Size: 1500 * gb},
	}
	for i := 0; i < 50; i++ {
		items = append(items, &domain.Item{Path: fmt.Sprintf("tv/s%d", i), Location: "/mnt/disk1", Size: 100 * gb})
		items = append(items, &domain.Item{Path: fmt.Sprintf("tv/t%d", i), Location: "/mnt/disk2", Size: 100 * gb})
	}

	result := Level(disks, items, LevelTolerance)

	if len(result.Split) != 1 || result.Split[0] != "tv/Billions" {
		t.Fatalf("expected Billions to be reported as split, got %v", result.Split)
	}

	// wherever Billions goes, all of it must land on one disk
	final := map[string]string{}
	for _, item := range items {
		if item.Path == "tv/Billions" {
			final[item.Location] = item.Location
		}
	}
	for _, m := range result.Moves {
		if m.Item.Path == "tv/Billions" {
			final[m.Item.Location] = m.Dst
		}
	}
	dests := map[string]bool{}
	for _, d := range final {
		dests[d] = true
	}
	if len(dests) > 1 {
		// left in place on both disks is fine; partially moved is not
		moved := 0
		for _, m := range result.Moves {
			if m.Item.Path == "tv/Billions" {
				moved++
			}
		}
		if moved > 0 {
			t.Fatalf("Billions was split by the plan: %v", final)
		}
	}

	if s := spread(fills(disks, result.Moves)); s > 0.05 {
		t.Fatalf("disks not levelled, spread %f", s)
	}
}

func TestLevelRespectsReservedSpace(t *testing.T) {
	disks := []LevelDisk{
		{Path: "/mnt/disk1", Size: 10 * tb, Free: 0},
		{Path: "/mnt/disk2", Size: 1 * tb, Free: 1 * tb, Reserved: 900 * gb},
	}
	items := []*domain.Item{
		{Path: "movies/big", Location: "/mnt/disk1", Size: 500 * gb},
		{Path: "movies/small", Location: "/mnt/disk1", Size: 50 * gb},
	}

	result := Level(disks, items, LevelTolerance)

	for _, m := range result.Moves {
		if m.Item.Path == "movies/big" {
			t.Fatalf("moved an item that breaks the reserve")
		}
	}
	if len(result.Moves) != 1 {
		t.Fatalf("expected only the small item to move, got %d moves", len(result.Moves))
	}
}

func TestLevelNoMovesWhenBalanced(t *testing.T) {
	disks := []LevelDisk{
		{Path: "/mnt/disk1", Size: 10 * tb, Free: 5 * tb},
		{Path: "/mnt/disk2", Size: 10 * tb, Free: 5 * tb},
	}
	items := []*domain.Item{
		{Path: "tv/a", Location: "/mnt/disk1", Size: 100 * gb},
		{Path: "tv/b", Location: "/mnt/disk2", Size: 100 * gb},
	}

	if result := Level(disks, items, LevelTolerance); len(result.Moves) != 0 {
		t.Fatalf("expected no moves on balanced disks, got %d", len(result.Moves))
	}
}

func TestLevelDiskNeverGivesAndReceives(t *testing.T) {
	// the screenshot case: ten busy disks, eleven nearly empty ones
	pct := []float64{80, 72, 38, 79, 63, 63, 73, 73, 38, 63, 9, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	disks := make([]LevelDisk, 0)
	items := make([]*domain.Item, 0)
	for i, p := range pct {
		path := fmt.Sprintf("/mnt/disk%d", i+1)
		used := uint64(p / 100 * float64(8*tb))
		disks = append(disks, LevelDisk{Path: path, Size: 8 * tb, Free: 8*tb - used, Reserved: gb})
		// most of the used space is shows of varying sizes
		for j := 0; uint64(j)*37*gb < used*9/10; j++ {
			items = append(items, &domain.Item{Path: fmt.Sprintf("tv/d%d-show%d", i, j), Location: path, Size: uint64(20+j%40) * gb})
		}
	}

	result := Level(disks, items, LevelTolerance)

	gives, gets := map[string]bool{}, map[string]bool{}
	for _, m := range result.Moves {
		gives[m.Item.Location] = true
		gets[m.Dst] = true
	}
	for d := range gives {
		if gets[d] {
			t.Fatalf("%s both gives and receives", d)
		}
	}

	if s := spread(fills(disks, result.Moves)); s > 0.03 {
		t.Fatalf("disks not levelled, spread %f", s)
	}
}
