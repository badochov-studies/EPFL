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

func (lps *loopPipScheduler) schedule() *blockBundles {
	bundles, II := lps.doSchedule()
	bundles = lps.allocateRegister(bundles)
	return lps.prepLoop(bundles, II)
}

func (lps *loopPipScheduler) doSchedule() (*blockBundles, int) {
	bundles := lps.doScheduleBB0(new(blockBundles))
	bundles, II := lps.doScheduleBB1(bundles)
	bundles = lps.doScheduleBB2(bundles)
	return bundles, II
}

func (lps *loopPipScheduler) doScheduleBB0(bundles *blockBundles) *blockBundles {
	return lps.scheduleBlockWithoutInterloopDeps(bundles, lps.deps.bb0(), bundles.bb0Start())
}

func (lps *loopPipScheduler) doScheduleBB1(bundles *blockBundles) (*blockBundles, int) {
	II := lps.getMinII(lps.deps.bb0())

	for {
		var ok bool
		bundles, ok = lps.tryScheduleBB1(bundles, II)
		if ok && lps.checkInterLoopDeps(bundles.bb1Start(), II) {
			return bundles, II
		}
		bundles.bb1 = nil
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

func (lps *loopPipScheduler) tryScheduleBB1(bundles *blockBundles, II int) (*blockBundles, bool) {
	depsBB1 := lps.deps.bb1()
	if len(depsBB1) == 0 {
		return bundles, true
	}

	// Allocate memory for new bundles.
	maxNewBundles := II * 5 // There are 2 ALU slots, 1 Mult, 1 Mem and 1 branch slot in each bundle.
	bundles.bb1 = make([]bundle, maxNewBundles)

	maxAssignedIdx := bundles.bb1Start() - 1
	loopDep := depsBB1[len(depsBB1)-1]

	for _, dep := range depsBB1[:len(depsBB1)-1] {
		// Check deps.
		minDepIdx := bundles.bb1Start()
		for _, depPc := range dep.nonInterloopBodyDeps() {
			bDep := lps.pcToBundle[depPc]
			latency := lps.instrs[depPc].latency()

			earliestTime := bDep + latency
			minDepIdx = maxInt(minDepIdx, earliestTime)
		}

		sI := &specIns{
			pred:  nil,
			instr: lps.instrs[dep.pc],
		}

		// Find place for the op.
		i := minDepIdx
		slot := noSlot
		for i < bundles.len() {
			slot = bundles.get(i).addInst(sI)
			if slot != noSlot {
				lps.pcToBundle[dep.pc] = i
				break
			}
			i++
		}
		// Slot not found, we have to increase II.
		if slot == noSlot {
			return bundles, false
		}
		maxAssignedIdx = maxInt(maxAssignedIdx, i)
		// Mark slots in other stages as taken.
		inStageIdx := (i - bundles.bb1Start()) % II
		for j := bundles.bb1Start() + inStageIdx; j < bundles.len(); j += II {
			bundles.get(j).vTakeSlot(slot)
		}
	}
	// TODO: remove bubble?
	// Add loop.
	loopBundle := bundles.bb1Start() + II - 1
	bundles.get(loopBundle)[branch] = &specIns{
		pred: nil,
		instr: instruction{
			pc:    loopDep.pc,
			type_: loop,
			imm:   int64(bundles.bb1Start()),
		},
	}
	lps.pcToBundle[loopDep.pc] = loopBundle
	maxAssignedIdx = maxInt(maxAssignedIdx, loopBundle)

	// Align size with II.
	blockLength := maxAssignedIdx + 1 - bundles.bb1Start()
	blockLength += (II - (blockLength % II)) % II
	bundles.shrinkBlock(bundles.bb1Start(), blockLength)

	return bundles, true
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

func (lps *loopPipScheduler) doScheduleBB2(bundles *blockBundles) *blockBundles {
	return lps.scheduleBlockWithoutInterloopDeps(bundles, lps.deps.bb2(), bundles.bb2Start())
}

func (lps *loopPipScheduler) allocateRegister(bundles *blockBundles) *blockBundles {
	return bundles
}

func (lps *loopPipScheduler) prepLoop(bundles *blockBundles, II int) *blockBundles {
	bundles = lps.prepLoopInit(bundles, II)
	bundles = lps.prepLoopBody(bundles, II)

	return bundles
}

func (lps *loopPipScheduler) prepLoopInit(bundles *blockBundles, II int) *blockBundles {
	if len(bundles.bb1) == 0 {
		return bundles
	}

	if len(bundles.bb0) == 0 {
		bundles.extendBlockBy(bundles.bb0Start(), 1)
	}
	lastBB0Bundle := bundles.get(bundles.bb1Start() - 1)
	if lastBB0Bundle[alu1] != nil || lastBB0Bundle[alu2] != nil {
		bundles.extendBlockBy(bundles.bb0Start(), 1)
		lastBB0Bundle = bundles.get(bundles.bb1Start() - 1)
	}

	movP32True := &specIns{
		pred: nil,
		instr: instruction{
			pc:    -1,
			type_: mov,
			regA: reg{
				type_: predReg,
				num:   32,
			},
			pred:    true,
			usesReg: false,
		},
	}

	numStages := len(bundles.bb1) / II
	movEC := &specIns{
		pred: nil,
		instr: instruction{
			pc:    -1,
			type_: mov,
			regA: reg{
				type_: specialReg,
				num:   ecReg,
			},
			imm:     int64(numStages - 1),
			usesReg: false,
		},
	}

	lastBB0Bundle[alu1] = movP32True
	lastBB0Bundle[alu2] = movEC

	// Adjust branch address.
	for i := 0; i < bundles.len(); i++ {
		b := bundles.get(i)
		if b[branch] != nil {
			b[branch].instr.imm = int64(bundles.bb1Start())
		}
	}

	return bundles
}

func (lps *loopPipScheduler) prepLoopBody(bundles *blockBundles, II int) *blockBundles {
	newBB1 := make([]bundle, II)

	for i, b := range bundles.bb1 {
		for slot, sI := range b {
			if sI != nil && sI.instr.type_ != nop {
				newBB1[i%II][slot] = sI
			}
		}
	}

	bundles.bb1 = newBB1
	return bundles
}
