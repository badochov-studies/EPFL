package scheduler

type loopScheduler struct {
	instrs     []instruction
	deps       sectionDeps
	pcToBundle []int
}

func newLoopScheduler(instrs []instruction, deps sectionDeps) *loopScheduler {
	return &loopScheduler{
		instrs:     instrs,
		deps:       deps,
		pcToBundle: make([]int, len(instrs)),
	}
}

func (ls *loopScheduler) schedule() []bundle {
	bundles := ls.doSchedule()
	return ls.allocateRegister(bundles)
}

func (ls *loopScheduler) blockNonInterloopBodyDepsCheck(bundles []bundle, deps []dependency, blockStartIdx int) []bundle {
	for _, dep := range deps {
		// Check deps.
		minDepIdx := blockStartIdx
		for _, depPc := range dep.nonInterloopBodyDeps() {
			bDep := ls.pcToBundle[depPc]
			latency := ls.instrs[depPc].latency()

			earliestTime := bDep + latency
			minDepIdx = maxInt(minDepIdx, earliestTime)
		}

		sI := &specIns{
			pred: nil,
			ins:  ls.instrs[dep.pc],
		}

		// Find place for the op.
		idx := minDepIdx
		for {
			// Extend slice if needed.
			for len(bundles) <= idx {
				bundles = append(bundles, bundle{})
			}

			if bundles[idx].addInst(sI) {
				ls.pcToBundle[dep.pc] = idx
				break
			}
			idx++
		}
	}

	return bundles
}

func (ls *loopScheduler) doSchedule() []bundle {
	bundles := ls.doScheduleBB0(nil)
	bundles = ls.doScheduleBB1(bundles)
	return ls.doScheduleBB2(bundles)
}

func (ls *loopScheduler) doScheduleBB0(bundles []bundle) []bundle {
	return ls.blockNonInterloopBodyDepsCheck(bundles, ls.deps.bb0(), len(bundles))
}

func (ls *loopScheduler) doScheduleBB1(bundles []bundle) []bundle {
	bb1 := ls.deps.bb1()
	if len(bb1) == 0 {
		return bundles
	}

	bb1BundlesStart := len(bundles)
	bb1NoLoop, bb1Loop := bb1[:len(bb1)-1], bb1[len(bb1)-1]
	bundles = ls.blockNonInterloopBodyDepsCheck(bundles, bb1NoLoop, bb1BundlesStart)

	// Check interloop deps.
	neededII := ls.getNeededII(bb1NoLoop)

	loopIdx := bb1BundlesStart + neededII - 1
	for len(bundles) <= loopIdx {
		bundles = append(bundles, bundle{})
	}

	loopStart := ls.removeBubble(bundles, bb1BundlesStart, neededII)

	bundles[len(bundles)-1].branch = &specIns{
		pred: nil,
		ins: instruction{
			pc:    bb1Loop.pc,
			type_: loop,
			imm:   int64(loopStart),
		},
	}
	ls.pcToBundle[bb1Loop.pc] = len(bundles) - 1

	return bundles
}

func (ls *loopScheduler) getNeededII(bb1NoLoop []dependency) int {
	// Check interloop deps.
	neededII := 1
	for _, dep := range bb1NoLoop {
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

func (ls *loopScheduler) removeBubble(bundles []bundle, bb1BundlesStart int, neededII int) int {
	loopStart := bb1BundlesStart
	for loopStart+neededII < len(bundles) {
		if bundles[loopStart].empty() {
			loopStart++
		} else {
			break
		}
	}

	return loopStart
}

func (ls *loopScheduler) doScheduleBB2(bundles []bundle) []bundle {
	return ls.blockNonInterloopBodyDepsCheck(bundles, ls.deps.bb2(), len(bundles))
}

func (ls *loopScheduler) allocateRegister(bundles []bundle) []bundle {
	currRegNum := uint8(1)

	instrs := ls.gatherInstrs(bundles)

	bundles, currRegNum = ls.allocateRegisterPhase1(bundles, currRegNum)
	ls.allocateRegisterPhase2(instrs)
	bundles = ls.allocateRegisterPhase3(bundles, instrs)
	bundles, _ = ls.allocateRegisterPhase4(bundles, currRegNum)
	return bundles
}

func (ls *loopScheduler) gatherInstrs(bundles []bundle) []*instruction {
	instrs := make([]*instruction, len(ls.pcToBundle))
	for _, b := range bundles {
		for _, sI := range b.allSpecInstr() {
			if sI != nil {
				instr := &sI.ins
				instrs[instr.pc] = instr
			}
		}
	}
	return instrs
}

func (ls *loopScheduler) allocateRegisterPhase1(bundles []bundle, currRegNum uint8) ([]bundle, uint8) {
	for _, b := range bundles {
		for _, sI := range b.allSpecInstr() {
			if sI != nil {
				dst, _ := sI.ins.mutRegs()
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

func (ls *loopScheduler) getLoopBundle(bundles []bundle) int {
	for i, b := range bundles {
		if b.branch != nil {
			return i
		}
	}
	return -1
}

func (ls *loopScheduler) allocateRegisterPhase3(bundles []bundle, instrs []*instruction) []bundle {
	loopBundle := ls.getLoopBundle(bundles)

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
				ins: instruction{
					pc:      -1,
					type_:   mov,
					regA:    *bb0Dst,
					regB:    *bb1Dst,
					usesReg: true,
				},
			}

			fixUpBundleIdx := ls.pcToBundle[bb1DepPc] + bb1DepInstr.latency()

			// Try to insert at last possible spot.
			if fixUpBundleIdx <= loopBundle {
				if !bundles[loopBundle].addInst(sI) {
					fixUpBundleIdx = loopBundle + 1
				}
			}

			// More space for bb1 is needed.
			if fixUpBundleIdx > loopBundle {
				bundles = append(bundles, make([]bundle, fixUpBundleIdx-loopBundle)...)
				copy(bundles[fixUpBundleIdx+1:], bundles[loopBundle+1:])
				bundles[fixUpBundleIdx] = bundle{
					alu1:   sI,
					branch: bundles[loopBundle].branch,
				}
				// Remove old branch instruction
				bundles[loopBundle].branch = nil

				loopBundle = fixUpBundleIdx
			}
		}
	}

	return bundles
}

func (ls *loopScheduler) allocateRegisterPhase4(bundles []bundle, currRegNum uint8) ([]bundle, uint8) {
	wroteTo := make(map[reg]bool)
	for _, b := range bundles {
		bundleWroteTo := make(map[reg]bool)
		for _, sI := range b.allSpecInstr() {
			if sI != nil {
				dst, ops := sI.ins.mutRegs()
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
