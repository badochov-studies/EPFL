package scheduler

type loopScheduler struct {
	blockScheduler
}

func newLoopScheduler(instrs []instruction, deps sectionDeps) *loopScheduler {
	return &loopScheduler{
		blockScheduler{
			instrs:     instrs,
			deps:       deps,
			pcToBundle: make([]int, len(instrs)),
		},
	}
}

func (ls *loopScheduler) schedule() *blockBundles {
	bundles := ls.doSchedule()
	return ls.allocateRegister(bundles)
}

func (ls *loopScheduler) doSchedule() *blockBundles {
	bundles := ls.doScheduleBB0(new(blockBundles))
	bundles = ls.doScheduleBB1(bundles)
	return ls.doScheduleBB2(bundles)
}

func (ls *loopScheduler) doScheduleBB0(bundles *blockBundles) *blockBundles {
	return ls.scheduleBlockWithoutInterloopDeps(bundles, ls.deps.bb0(), bundles.bb0Start())
}

func (ls *loopScheduler) doScheduleBB1(bundles *blockBundles) *blockBundles {
	bb1 := ls.deps.bb1()
	if len(bb1) == 0 {
		return bundles
	}

	bb1NoLoop, bb1Loop := bb1[:len(bb1)-1], bb1[len(bb1)-1]
	bundles = ls.scheduleBlockWithoutInterloopDeps(bundles, bb1NoLoop, bundles.bb1Start())

	// Check interloop deps.
	neededII := ls.getNeededII()

	loopNeededIdx := bundles.bb1Start() + neededII - 1
	bundles.extend(bundles.bb1Start(), loopNeededIdx+1)

	loopStart := ls.removeBubble(bundles, neededII)

	loopIdx := bundles.len() - 1
	bundles.get(loopIdx)[branch] = &specIns{
		pred: nil,
		instr: instruction{
			pc:    bb1Loop.pc,
			type_: loop,
			imm:   int64(loopStart),
		},
	}
	ls.pcToBundle[bb1Loop.pc] = loopIdx

	return bundles
}

func (ls *loopScheduler) getNeededII() int {
	// Check interloop deps.
	neededII := 1
	for _, dep := range ls.deps.bb1() {
		currBundle := ls.pcToBundle[dep.pc]

		for _, iDep := range dep.interloopDeps {
			depPc := iDep.body
			latency := ls.instrs[depPc].latency()
			depBundle := ls.pcToBundle[depPc]

			depNeededII := depBundle + latency - currBundle
			neededII = maxInt(neededII, depNeededII)
		}
	}

	return neededII
}

func (ls *loopScheduler) removeBubble(bundles *blockBundles, neededII int) int {
	loopStart := bundles.bb1Start()
	for loopStart+neededII < bundles.len() {
		if bundles.get(loopStart).empty() {
			loopStart++
		} else {
			break
		}
	}

	return loopStart
}

func (ls *loopScheduler) doScheduleBB2(bundles *blockBundles) *blockBundles {
	return ls.scheduleBlockWithoutInterloopDeps(bundles, ls.deps.bb2(), bundles.bb2Start())
}

func (ls *loopScheduler) allocateRegister(bundles *blockBundles) *blockBundles {
	currRegNum := uint8(1)

	instrs := ls.gatherInstrs(bundles)

	bundles, currRegNum = ls.allocateRegisterPhase1(bundles, currRegNum)
	ls.allocateRegisterPhase2(instrs)
	bundles = ls.allocateRegisterPhase3(bundles, instrs)
	bundles, _ = ls.allocateRegisterPhase4(bundles, currRegNum)
	return bundles
}

func (ls *loopScheduler) gatherInstrs(bundles *blockBundles) []*instruction {
	instrs := make([]*instruction, len(ls.pcToBundle))
	for i := 0; i < bundles.len(); i++ {
		for _, sI := range bundles.get(i) {
			if sI != nil {
				instr := &sI.instr
				instrs[instr.pc] = instr
			}
		}
	}
	return instrs
}

func (ls *loopScheduler) allocateRegisterPhase1(bundles *blockBundles, currRegNum uint8) (*blockBundles, uint8) {
	for i := 0; i < bundles.len(); i++ {
		for _, sI := range bundles.get(i) {
			if sI != nil {
				dst, _ := sI.instr.mutRegs()
				if dst != nil && dst.type_ == xReg {
					dst.num = currRegNum
					currRegNum++
				}
			}
		}
	}
	return bundles, currRegNum
}

func (ls *loopScheduler) allocateRegisterPhase2(instrs []*instruction) {
	for i, dep := range ls.deps.deps {
		instr := instrs[i]

		_, ops := instr.mutRegs()
		for r, depPc := range dep.nonInterloopBodyDeps() {
			for _, op := range ops {
				if *op == r {
					dst, _ := instrs[depPc].regs()
					*op = *dst
				}
			}
		}
	}
}

func (ls *loopScheduler) allocateRegisterPhase3(bundles *blockBundles, instrs []*instruction) *blockBundles {
	loopBundleIdx := getLoopBundleIdx(bundles)

	// TODO: this can be optimized by better choosing the order of the fixups.
	handled := make(map[reg]bool)
	for _, dep := range ls.deps.deps {
		for _, iDep := range dep.interloopDeps {
			bb0DepPc := iDep.init
			bb0DepInstr := instrs[bb0DepPc]
			bb0Dst, _ := bb0DepInstr.regs()

			if handled[*bb0Dst] {
				continue
			} else {
				handled[*bb0Dst] = true
			}

			bb1DepPc := iDep.body
			bb1DepInstr := instrs[bb1DepPc]
			bb1Dst, _ := bb1DepInstr.regs()

			sI := &specIns{
				pred: nil,
				instr: instruction{
					pc:      -1,
					type_:   mov,
					regA:    *bb0Dst,
					regB:    *bb1Dst,
					usesReg: true,
				},
			}

			fixUpBundleIdx := ls.pcToBundle[bb1DepPc] + bb1DepInstr.latency()

			// Try to insert at last possible spot.
			// TODO: what is a last possible spot? Is it last bundle of the loop, or last viable bundle?
			if fixUpBundleIdx <= loopBundleIdx {
				if bundles.get(loopBundleIdx).addInst(sI) == noSlot {
					fixUpBundleIdx = loopBundleIdx + 1
				}
			}

			// More space for bb1 is needed.
			if fixUpBundleIdx > loopBundleIdx {
				bundles.extendBlockBy(bundles.bb1Start(), fixUpBundleIdx-loopBundleIdx)

				loopBundle := bundles.get(loopBundleIdx)
				*bundles.get(fixUpBundleIdx) = bundle{
					alu1:   sI,
					branch: loopBundle[branch],
				}
				// Remove old branch instruction
				loopBundle[branch] = nil

				loopBundleIdx = fixUpBundleIdx
			}
		}
	}

	return bundles
}

func (ls *loopScheduler) allocateRegisterPhase4(bundles *blockBundles, currRegNum uint8) (*blockBundles, uint8) {
	wroteTo := make(map[reg]bool)
	for i := 0; i < bundles.len(); i++ {
		bundleWroteTo := make(map[reg]bool)
		for _, sI := range bundles.get(i) {
			if sI != nil {
				dst, ops := sI.instr.mutRegs()
				if dst != nil {
					bundleWroteTo[*dst] = true
				}
				for _, op := range ops {
					if op.type_ == xReg && !wroteTo[*op] {
						op.num = currRegNum
						currRegNum++
					}
				}
			}
		}

		for r := range bundleWroteTo {
			wroteTo[r] = true
		}
	}

	return bundles, currRegNum
}
