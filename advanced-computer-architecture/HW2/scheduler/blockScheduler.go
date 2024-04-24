package scheduler

type blockScheduler struct {
	instrs     []instruction
	deps       sectionDeps
	pcToBundle []int
}

func (bs *blockScheduler) scheduleBlockWithoutInterloopDeps(bundles *blockBundles, deps []dependency, blockStartIdx int) *blockBundles {
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
			pred:  nil,
			instr: bs.instrs[dep.pc],
		}

		// Find place for the op.
		idx := minDepIdx
		for {
			// Extend slice if needed.
			bundles.extend(blockStartIdx, idx+1)

			if bundles.get(idx).addInst(sI) != noSlot {
				bs.pcToBundle[dep.pc] = idx
				break
			}
			idx++
		}
	}
	return bundles
}
