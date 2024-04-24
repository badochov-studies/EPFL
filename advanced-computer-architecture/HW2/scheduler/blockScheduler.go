package scheduler

type blockScheduler struct {
	instrs     []instruction
	deps       sectionDeps
	pcToBundle []int
}

func (bs *blockScheduler) scheduleBlockWithoutInterloopDeps(bundles []bundle, deps []dependency, blockStartIdx int) []bundle {
	for _, dep := range deps {
		// Check deps.
		minDepIdx := blockStartIdx
		for _, depPc := range dep.nonInterloopBodyDeps() {
			bDep := bs.pcToBundle[depPc]
			latency := bs.instrs[depPc].latency()

			earliestTime := bDep + latency
			minDepIdx = maxInt(minDepIdx, earliestTime)
		}

		sI := &specIns{
			pred: nil,
			ins:  bs.instrs[dep.pc],
		}

		// Find place for the op.
		idx := minDepIdx
		for {
			// Extend slice if needed.
			for len(bundles) <= idx {
				bundles = append(bundles, bundle{})
			}

			if bundles[idx].addInst(sI) != noSlot {
				bs.pcToBundle[dep.pc] = idx
				break
			}
			idx++
		}
	}

	return bundles
}
