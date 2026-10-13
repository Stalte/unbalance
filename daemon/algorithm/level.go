package algorithm

import (
	"sort"

	"unbalance/daemon/domain"
)

// LevelTolerance is how far (as a fraction of disk size) a disk may sit above
// the average fill and still count as levelled. It keeps the planner from
// chasing the last fraction of a percent with lots of tiny moves.
const LevelTolerance = 0.01

// LevelDisk is a disk taking part in a level operation.
type LevelDisk struct {
	Path     string
	Size     uint64
	Free     uint64
	Reserved uint64 // free space that must remain after receiving data
}

// LevelMove sends one item from its current disk (Item.Location) to Dst.
type LevelMove struct {
	Item *domain.Item
	Dst  string
}

// LevelResult is the outcome of a level plan.
type LevelResult struct {
	// Target is the fill fraction every disk is aimed at
	Target float64
	Moves  []LevelMove
	// Split lists the folders found on more than one of the selected disks
	Split []string
}

// a unit is everything that must stay together: all the parts of one folder
// (e.g. tvshows/Billions), wherever they currently live
type levelUnit struct {
	path    string
	parts   []*domain.Item
	size    uint64
	primary int // disk holding the largest part
	moved   bool
}

const (
	roleNone = iota
	roleDonor
	roleReceiver
)

// Level plans moves that bring every disk as close as possible to the same
// percentage full. Items sharing a path are a unit and always end up on a
// single disk, so a tv show is never split by the plan. Each step makes the
// one move, off any disk above the average, that most reduces the spread of
// fill percentages (sum of squared deviations from the average). A disk
// either gives or receives data, never both, so nothing is moved twice.
func Level(disks []LevelDisk, items []*domain.Item, tolerance float64) LevelResult {
	result := LevelResult{Moves: make([]LevelMove, 0), Split: make([]string, 0)}

	n := len(disks)
	if n < 2 {
		return result
	}

	index := make(map[string]int, n)
	size := make([]float64, n)
	used := make([]float64, n)
	free := make([]float64, n)
	reserved := make([]float64, n)
	role := make([]int, n)

	var totalUsed, totalSize float64
	for i, disk := range disks {
		index[disk.Path] = i
		size[i] = float64(disk.Size)
		free[i] = float64(disk.Free)
		used[i] = float64(disk.Size) - float64(disk.Free)
		reserved[i] = float64(disk.Reserved)
		totalUsed += used[i]
		totalSize += size[i]
	}

	if totalSize == 0 {
		return result
	}

	target := totalUsed / totalSize
	result.Target = target

	dev := func(i int) float64 {
		if size[i] == 0 {
			return 0
		}
		return used[i]/size[i] - target
	}

	// group items into units, keeping the order stable for reproducible plans
	units := make([]*levelUnit, 0)
	byPath := make(map[string]*levelUnit)
	for _, item := range items {
		if _, ok := index[item.Location]; !ok {
			continue
		}

		unit, ok := byPath[item.Path]
		if !ok {
			unit = &levelUnit{path: item.Path}
			byPath[item.Path] = unit
			units = append(units, unit)
		}
		unit.parts = append(unit.parts, item)
		unit.size += item.Size
	}

	byDisk := make([][]*levelUnit, n)
	for _, unit := range units {
		var largest uint64
		for _, part := range unit.parts {
			if part.Size >= largest {
				largest = part.Size
				unit.primary = index[part.Location]
			}
		}
		if len(unit.parts) > 1 {
			result.Split = append(result.Split, unit.path)
		}
		byDisk[unit.primary] = append(byDisk[unit.primary], unit)
	}

	// larger units first: on equal gain, fewer bigger moves beat many small ones
	for _, list := range byDisk {
		sort.SliceStable(list, func(i, j int) bool { return list[i].size > list[j].size })
	}

	// gain returns how much moving unit to disk t reduces the spread, or false
	// if the move isn't allowed
	gain := func(unit *levelUnit, t int) (float64, bool) {
		// fast path for the common case of a folder living on one disk
		if len(unit.parts) == 1 {
			s := index[unit.parts[0].Location]
			x := float64(unit.parts[0].Size)
			if s == t || role[s] == roleReceiver || free[t]-x < reserved[t] {
				return 0, false
			}
			ds, dt := dev(s), dev(t)
			ns, nt := ds-x/size[s], dt+x/size[t]
			return ds*ds + dt*dt - ns*ns - nt*nt, true
		}

		var incoming float64
		// at most a handful of disks are touched; deltas holds per-disk byte changes
		deltas := map[int]float64{}
		for _, part := range unit.parts {
			s := index[part.Location]
			if s == t {
				continue
			}
			if role[s] == roleReceiver {
				return 0, false
			}
			deltas[s] -= float64(part.Size)
			incoming += float64(part.Size)
		}

		if incoming == 0 {
			return 0, false
		}

		if free[t]-incoming < reserved[t] {
			return 0, false
		}

		deltas[t] += incoming

		var before, after float64
		for d, delta := range deltas {
			old := dev(d)
			updated := old + delta/size[d]
			before += old * old
			after += updated * updated
		}

		return before - after, true
	}

	const minGain = 1e-12

	for {
		// pick the single best move across every overfilled disk
		var best *levelUnit
		bestTarget := -1
		bestGain := minGain

		for donor := range disks {
			if role[donor] == roleReceiver || dev(donor) <= tolerance {
				continue
			}

			for _, unit := range byDisk[donor] {
				if unit.moved {
					continue
				}
				for t := range disks {
					if t == donor || role[t] == roleDonor || dev(t) >= 0 {
						continue
					}
					g, ok := gain(unit, t)
					if ok && g > bestGain {
						best, bestTarget, bestGain = unit, t, g
					}
				}
			}
		}

		if best == nil {
			break
		}

		best.moved = true
		role[bestTarget] = roleReceiver
		for _, part := range best.parts {
			s := index[part.Location]
			if s == bestTarget {
				continue
			}
			role[s] = roleDonor
			used[s] -= float64(part.Size)
			free[s] += float64(part.Size)
			used[bestTarget] += float64(part.Size)
			free[bestTarget] -= float64(part.Size)
			result.Moves = append(result.Moves, LevelMove{Item: part, Dst: disks[bestTarget].Path})
		}
	}

	return result
}
