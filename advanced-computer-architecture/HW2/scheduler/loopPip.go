package scheduler

type loopPipScheduler struct {
	blockScheduler
	dstAllocated []bool
	opAllocated  [][2]bool
}

func newLoopPipScheduler(instrs []instruction, deps sectionDeps) *loopPipScheduler {
	return &loopPipScheduler{
		blockScheduler: blockScheduler{
			instrs:     instrs,
			deps:       deps,
			pcToBundle: make([]int, len(instrs)),
		},
		dstAllocated: make([]bool, len(instrs)),
		opAllocated:  make([][2]bool, len(instrs)),
	}
}

func (lps *loopPipScheduler) schedule() *blockBundles {
	bundles, II := lps.doSchedule()
	bundles = lps.allocateRegister(bundles, II)
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
	if len(lps.deps.bb1()) == 0 {
		return bundles, 0
	}

	II := lps.getMinII(lps.deps.bb1())

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
	maxNewBundles := II*5 + 3 // There are 2 ALU slots, 1 Mult, 1 Mem and 1 branch slot in each bundle, 3 for potential bubble.
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

	// Remove bubble.

	// Check empty loop case.
	if maxAssignedIdx >= bundles.bb1Start() {
		bubbleSize := 0
		for bundles.get(bundles.bb1Start() + bubbleSize).empty() {
			bubbleSize++
		}
		bundles.extendBlockBy(bundles.bb0Start(), bubbleSize)
		bundles.trimStart(bundles.bb1Start(), bubbleSize)
	}

	// Add loop.
	loopBundle := bundles.bb1Start() + II - 1
	bundles.get(loopBundle)[branch] = &specIns{
		pred: nil,
		instr: instruction{
			pc:    loopDep.pc,
			type_: loopPip,
			imm:   int64(bundles.bb1Start()),
		},
	}
	lps.pcToBundle[loopDep.pc] = loopBundle
	maxAssignedIdx = maxInt(maxAssignedIdx, loopBundle)

	// Shrink bb1 size.
	blockLength := maxAssignedIdx + 1 - bundles.bb1Start()
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

func (lps *loopPipScheduler) allocateRegister(bundles *blockBundles, II int) *blockBundles {
	currStaticRegNum := uint8(1)
	currRotRegNum := uint8(32)

	instrs := lps.gatherInstrs(bundles)

	currRotRegNum = lps.allocateRegisterPhase1(bundles, currRotRegNum, II)
	currStaticRegNum = lps.allocateRegisterPhase2(bundles, instrs, currStaticRegNum)
	lps.allocateRegisterPhase3(bundles, instrs, II)
	currStaticRegNum = lps.allocateRegisterPhase4(bundles, instrs, currStaticRegNum, II)

	_, _ = currStaticRegNum, instrs

	return bundles
}

func (lps *loopPipScheduler) gatherInstrs(bundles *blockBundles) []*instruction {
	instrs := make([]*instruction, len(lps.pcToBundle))
	for i := 0; i < bundles.len(); i++ {
		for _, sI := range bundles.get(i) {
			if sI != nil && sI.instr.pc != -1 {
				instr := &sI.instr
				instrs[instr.pc] = instr
			}
		}
	}
	return instrs
}

func (lps *loopPipScheduler) allocateRegisterPhase1(bundles *blockBundles, currRotRegNum uint8, II int) uint8 {
	if len(bundles.bb1) == 0 {
		return currRotRegNum
	}
	numStages := uint8(loopStages(bundles, II))

	for _, b := range bundles.bb1 {
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				dst, _ := sI.instr.mutRegs()
				if dst != nil && dst.type_ == xReg {
					dst.num = currRotRegNum
					currRotRegNum += numStages + 1

					lps.dstAllocated[sI.instr.pc] = true
				}
			}
		}
	}
	return currRotRegNum
}

func (lps *loopPipScheduler) allocateRegisterPhase2(bundles *blockBundles, instrs []*instruction, currStaticRegNum uint8) uint8 {
	for _, b := range bundles.bb1 {
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				dep := lps.deps.deps[sI.instr.pc]

				for _, depPc := range dep.loopInvariantDeps {
					if !lps.dstAllocated[depPc] {
						dst, _ := instrs[depPc].mutRegs()
						dst.num = currStaticRegNum
						currStaticRegNum++

						lps.dstAllocated[depPc] = true
					}
				}
			}
		}
	}

	return currStaticRegNum
}

func (lps *loopPipScheduler) allocateRegisterPhase3(bundles *blockBundles, instrs []*instruction, II int) {
	for _, b := range bundles.bb1 {
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				instr := instrs[sI.instr.pc]
				dep := lps.deps.deps[sI.instr.pc]
				_, ops := instr.mutRegs()
				stage := lps.loopStage(bundles, sI.instr.pc, II)

				for r, depPc := range dep.loopInvariantDeps {
					for i, op := range ops {
						if *op == r {
							dst, _ := instrs[depPc].regs()
							*op = *dst

							lps.opAllocated[sI.instr.pc][i] = true
						}
					}
				}

				for r, depPc := range dep.localDeps {
					for i, op := range ops {
						if *op == r {
							dst, _ := instrs[depPc].regs()
							stageDep := lps.loopStage(bundles, depPc, II)
							op.num = dst.num + uint8(stage-stageDep)

							lps.opAllocated[sI.instr.pc][i] = true
						}
					}
				}

				for r, iDep := range dep.interloopDeps {
					for i, op := range ops {
						if *op == r {
							depPc := iDep.body
							dst, _ := instrs[depPc].regs()
							stageDep := lps.loopStage(bundles, depPc, II)
							op.num = dst.num + uint8(stage-stageDep) + 1

							lps.opAllocated[sI.instr.pc][i] = true
						}
					}
				}
			}
		}
	}
}

func (lps *loopPipScheduler) loopStage(bundles *blockBundles, pc int, II int) int {
	bundlesIdx := lps.pcToBundle[pc]
	inLoopIdx := bundlesIdx - bundles.bb1Start()
	return inLoopIdx / II
}

func (lps *loopPipScheduler) allocateRegisterPhase4(bundles *blockBundles, instrs []*instruction, currStaticRegNum uint8, II int) uint8 {
	lps.allocateRegisterPhase4InterLoop(bundles, instrs, II)
	currStaticRegNum = lps.allocateRegisterPhase4LocalDeps(bundles, instrs, currStaticRegNum)
	lps.allocateRegisterPhase4PostDeps(bundles, instrs, II)
	lps.allocateRegisterPhase4LoopInvariants(bundles, instrs)
	currStaticRegNum = lps.allocateRegisterUnassignedRead(bundles, currStaticRegNum)

	return currStaticRegNum
}

func (lps *loopPipScheduler) allocateRegisterPhase4InterLoop(bundles *blockBundles, instrs []*instruction, II int) {
	initInterLoopDeps := lps.collectInitInterLoopDeps()
	for _, b := range bundles.bb0 {
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				dst, _ := sI.instr.mutRegs()
				if dst != nil {
					dep, ok := initInterLoopDeps[*dst]
					if ok && dep.init == sI.instr.pc {
						bodyStage := lps.loopStage(bundles, dep.body, II)
						bodyInstr := instrs[dep.body]
						bodyDst, _ := bodyInstr.regs()

						dst.num = bodyDst.num - uint8(bodyStage) + 1

						lps.dstAllocated[sI.instr.pc] = true
					}
				}
			}
		}
	}
}

func (lps *loopPipScheduler) allocateRegisterPhase4LocalDeps(bundles *blockBundles, instrs []*instruction, currStaticRegNum uint8) uint8 {
	for _, b := range bundles.bb0AndBB2() {
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				dst, ops := sI.instr.mutRegs()
				dep := lps.deps.deps[sI.instr.pc]

				// Update ops
				for depReg, depPc := range dep.localDeps {
					depInstr := instrs[depPc]
					depDst, _ := depInstr.regs()
					for i, op := range ops {
						if *op == depReg {
							*op = *depDst

							lps.opAllocated[sI.instr.pc][i] = true
						}
					}
				}

				// Update dst, if not changed previously.
				if dst != nil && dst.type_ != specialReg && !lps.dstAllocated[sI.instr.pc] {
					dst.num = currStaticRegNum
					currStaticRegNum++

					lps.dstAllocated[sI.instr.pc] = true
				}
			}
		}
	}
	return currStaticRegNum
}

func (lps *loopPipScheduler) allocateRegisterPhase4PostDeps(bundles *blockBundles, instrs []*instruction, II int) {
	for _, b := range bundles.bb2 {
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				_, ops := sI.instr.mutRegs()
				dep := lps.deps.deps[sI.instr.pc]
				stage := loopStages(bundles, II) - 1

				for depReg, depPc := range dep.postLoopDeps {
					depInstr := instrs[depPc]
					depDst, _ := depInstr.regs()
					depStage := lps.loopStage(bundles, depPc, II)

					for i, op := range ops {
						if *op == depReg {
							op.num = depDst.num + uint8(stage-depStage)

							lps.opAllocated[sI.instr.pc][i] = true
						}
					}
				}
			}
		}
	}
}

func (lps *loopPipScheduler) allocateRegisterPhase4LoopInvariants(bundles *blockBundles, instrs []*instruction) {
	for _, b := range bundles.bb0AndBB2() {
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				_, ops := sI.instr.mutRegs()
				dep := lps.deps.deps[sI.instr.pc]

				for depReg, depPc := range dep.loopInvariantDeps {
					depInstr := instrs[depPc]
					depDst, _ := depInstr.regs()

					for i, op := range ops {
						if *op == depReg {
							*op = *depDst

							lps.opAllocated[sI.instr.pc][i] = true
						}
					}
				}
			}
		}
	}
}

func (lps *loopPipScheduler) allocateRegisterUnassignedRead(bundles *blockBundles, currStaticRegNum uint8) uint8 {
	for i := 0; i < bundles.len(); i++ {
		b := bundles.get(i)
		for _, sI := range b {
			if sI != nil && sI.instr.pc != -1 {
				_, ops := sI.instr.mutRegs()

				for i, op := range ops {
					if !lps.opAllocated[sI.instr.pc][i] {
						op.num = currStaticRegNum
						currStaticRegNum++

						lps.opAllocated[sI.instr.pc][i] = true
					}
				}
			}
		}
	}
	return currStaticRegNum
}

func (lps *loopPipScheduler) collectInitInterLoopDeps() map[reg]struct{ init, body int } {
	initInterLoopDeps := make(map[reg]struct{ init, body int })
	for _, dep := range lps.deps.deps {
		for regDep, iDep := range dep.interloopDeps {
			initInterLoopDeps[regDep] = iDep
		}
	}
	return initInterLoopDeps
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

	numStages := loopStages(bundles, II)
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

	for _, sI := range []*specIns{movEC, movP32True} {
		for lastBB0Bundle.addInst(sI) == noSlot {
			bundles.extendBlockBy(bundles.bb0Start(), 1)
			lastBB0Bundle = bundles.get(bundles.bb1Start() - 1)
		}
	}

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
	if len(bundles.bb1) == 0 {
		return bundles
	}

	newBB1 := make([]bundle, II)

	for i, b := range bundles.bb1 {
		for slot, sI := range b {
			if sI != nil && sI.instr.type_ != nop {
				if !sI.instr.type_.isBranch() {
					stage := uint8(i / II)
					sI.pred = &reg{
						type_: predReg,
						num:   32 + stage,
					}
				}
				newBB1[i%II][slot] = sI
			}
		}
	}

	bundles.bb1 = newBB1
	return bundles
}
