package scheduler

type loopPipScheduler struct {
	blockScheduler
}

func newLoopPipScheduler(instrs []instruction, deps sectionDeps) *loopPipScheduler {
	return &loopPipScheduler{
		blockScheduler{
			instrs:     instrs,
			deps:       deps,
			pcToBundle: make([]int, len(instrs)),
		},
	}
}

func (lps *loopPipScheduler) schedule() []bundle {
	bundles := lps.doSchedule()
	bundles = lps.allocateRegister(bundles)
	return lps.prepLoop(bundles)
}

func (lps *loopPipScheduler) doSchedule() []bundle {
	bundles := lps.doScheduleBB0(nil)
	bundles = lps.doScheduleBB1(bundles)
	return lps.doScheduleBB2(bundles)
}

func (lps *loopPipScheduler) doScheduleBB0(bundles []bundle) []bundle {
	return lps.scheduleBlockWithoutInterloopDeps(bundles, lps.deps.bb0(), len(bundles))
}

func (lps *loopPipScheduler) doScheduleBB1(bundles []bundle) []bundle {
	II := lps.getMinII(lps.deps.bb0())

	for {
		newBundles, ok := lps.tryScheduleBB1(bundles, II)
		if ok && lps.checkInterLoopDeps(len(bundles), II) {
			return newBundles
		}
		II++
	}
}

func (lps *loopPipScheduler) getMinII(deps []dependency) int {
	var aluCounter, mulCounter, memCounter int

	for _, dep := range deps {
		instr := lps.instrs[dep.pc]
		if instr.type_.isMem() {
			memCounter++
		} else if instr.type_.isAlu() {
			aluCounter++
		} else if instr.type_.isMul() {
			mulCounter++
		}
	}
	// There are two alu units and we need a ceil.
	aluScore := (aluCounter + 1) / 2

	return maxInt(aluScore, mulCounter, memCounter)
}

func (lps *loopPipScheduler) tryScheduleBB1(oldBundles []bundle, II int) ([]bundle, bool) {
	if len(lps.deps.bb0()) == 0 {
		return oldBundles, true
	}

	maxNewBundles := II * 5 // There are 2 ALU slots, 1 Mult, 1 Mem and 1 branch slot in each bundle.

	blockStartIdx := len(oldBundles)
	maxAssignedIdx := len(oldBundles) - 1

	// Allocate memory for new bundles.
	bundles := make([]bundle, len(oldBundles)+maxNewBundles)
	copy(bundles, oldBundles)

	depsBB1 := lps.deps.bb1()
	loopDep := depsBB1[len(depsBB1)-1]

	for _, dep := range depsBB1[:len(depsBB1)-1] {
		// Check deps.
		minDepIdx := blockStartIdx
		for _, depPc := range dep.nonInterloopBodyDeps() {
			bDep := lps.pcToBundle[depPc]
			latency := lps.instrs[depPc].latency()

			earliestTime := bDep + latency
			minDepIdx = maxInt(minDepIdx, earliestTime)
		}

		sI := &specIns{
			pred: nil,
			ins:  lps.instrs[dep.pc],
		}

		// Find place for the op.
		i := minDepIdx
		slot := noSlot
		for i < len(bundles) {
			slot = bundles[i].addInst(sI)
			if slot != noSlot {
				lps.pcToBundle[dep.pc] = i
				break
			}
			i++
		}
		// Slot not found, we have to increase II.
		if slot == noSlot {
			return nil, false
		}
		maxAssignedIdx = maxInt(maxAssignedIdx, i)
		// Mark slots in other stages as taken.
		inStageIdx := (i - blockStartIdx) % II
		for j := blockStartIdx + inStageIdx; j < len(bundles); j += II {
			bundles[j].vTakeSlot(slot)
		}
	}
	// TODO: remove bubble?
	// Add loop.
	loopBundle := blockStartIdx + II - 1
	bundles[loopBundle][branch] = &specIns{
		pred: nil,
		ins: instruction{
			pc:    loopDep.pc,
			type_: loop,
			imm:   int64(blockStartIdx),
		},
	}
	lps.pcToBundle[loopDep.pc] = loopBundle
	maxAssignedIdx = maxInt(maxAssignedIdx, loopBundle)

	// Align size with II.
	maxAssignedIdx += (II - ((maxAssignedIdx + 1 - blockStartIdx) % II)) % II
	return bundles[:maxAssignedIdx+1], true
}

func (lps *loopPipScheduler) checkInterLoopDeps(bb1StartIdx int, ii int) bool {
	for _, dep := range lps.deps.bb1() {
		currBundle := lps.pcToBundle[dep.pc]
		inStageCurrIdx := (currBundle - bb1StartIdx) % ii

		for _, iDep := range dep.interloopDeps {
			depPc := iDep.body
			latency := lps.instrs[depPc].latency()
			depBundle := lps.pcToBundle[depPc]

			inStageDepIdx := (depBundle - bb1StartIdx) % ii

			if inStageDepIdx+latency > ii+inStageCurrIdx {
				return false
			}
		}
	}
	return true
}

func (lps *loopPipScheduler) doScheduleBB2(bundles []bundle) []bundle {
	return lps.scheduleBlockWithoutInterloopDeps(bundles, lps.deps.bb2(), len(bundles))
}

func (lps *loopPipScheduler) allocateRegister(bundles []bundle) []bundle {
	return bundles
}

func (lps *loopPipScheduler) prepLoop(bundles []bundle) []bundle {
	return bundles
}
