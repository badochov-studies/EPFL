package scheduler

import "fmt"

func (b *bundle) addInst(sI *specIns) bool {
	if sI.ins.type_.isMul() {
		if b.mult == nil {
			b.mult = sI
			return true
		}
	} else if sI.ins.type_.isAlu() {
		if b.alu1 == nil {
			b.alu1 = sI
			return true
		}
		if b.alu2 == nil {
			b.alu2 = sI
			return true
		}
	} else if sI.ins.type_.isMem() {
		if b.mem == nil {
			b.mem = sI
			return true
		}
	} else {
		panic(fmt.Sprint("Unexpected instruction type to add:", sI.ins.type_))
	}
	return false
}

func (s *Scheduler) loopSchedule(instrs []instruction, deps sectionDeps) (bundles []bundle, pcToBundle []int) {
	pcToBundle = make([]int, 0, len(instrs))

	getDepEarliestTime := func(depPc int) int {
		bDep := pcToBundle[depPc]
		latency := instrs[depPc].latency()

		return bDep + latency
	}

	// Schedule bb0.
	for i, dep := range deps.bb0 {
		// Check deps.
		minDepIdx := 0
		for _, depPc := range dep.localDeps {
			minDepIdx = maxInt(minDepIdx, getDepEarliestTime(depPc))
		}

		sI := &specIns{
			pred: nil,
			ins:  instrs[i],
		}

		// Find place for the op.
		idx := minDepIdx
		for {
			// Extend slice if needed.
			for len(bundles) <= idx {
				bundles = append(bundles, bundle{})
			}

			if bundles[idx].addInst(sI) {
				pcToBundle = append(pcToBundle, idx)
				break
			}
			idx++
		}
	}

	// Schedule bb1
	bb1MinDepIdx := len(bundles)
	bb1StartIdx := len(deps.bb0)
	if len(deps.bb1) > 0 {
		// Schedule without being concerned with II.
		for bb1Idx, dep := range deps.bb1 {
			i := bb1Idx + bb1StartIdx
			instr := instrs[i]

			if instr.type_.isBranch() {
				break
			}

			// Check deps.
			minDepIdx := bb1MinDepIdx
			// Check bb0 deps
			for _, depPc := range dep.loopInvariantDeps {
				minDepIdx = maxInt(minDepIdx, getDepEarliestTime(depPc))
			}
			for _, iDep := range dep.interloopDeps {
				minDepIdx = maxInt(minDepIdx, getDepEarliestTime(iDep.init))
			}
			// Check bb1 deps
			for _, depPc := range dep.localDeps {
				minDepIdx = maxInt(minDepIdx, getDepEarliestTime(depPc))
			}

			sI := &specIns{
				pred: nil,
				ins:  instr,
			}

			// Find place for the op.
			idx := minDepIdx
			for {
				// Extend slice if needed.
				for len(bundles) <= idx {
					bundles = append(bundles, bundle{})
				}

				if bundles[idx].addInst(sI) {
					pcToBundle = append(pcToBundle, idx)
					break
				}
				idx++
			}
		}
		// Check interloop deps.
		neededII := 1
		for bb1Idx, dep := range deps.bb1 {
			i := bb1Idx + len(deps.bb0)

			if instrs[i].type_.isBranch() {
				break
			}
			currBundle := pcToBundle[i]

			for _, iDep := range dep.interloopDeps {
				depPc := iDep.body
				latency := instrs[depPc].latency()
				depBundle := pcToBundle[depPc]

				depNeededII := depBundle + latency - currBundle
				neededII = maxInt(neededII, depNeededII)
			}
		}

		loopIdx := bb1MinDepIdx + neededII - 1
		for len(bundles) <= loopIdx {
			bundles = append(bundles, bundle{})
		}

		// Detect bubble
		loopStart := bb1MinDepIdx
		for loopStart+neededII < len(bundles) {
			if bundles[loopStart].empty() {
				loopStart++
			} else {
				break
			}
		}

		bundles[len(bundles)-1].branch = &specIns{
			pred: nil,
			ins: instruction{
				pc:    len(pcToBundle),
				type_: loop,
				imm:   int64(loopStart),
			},
		}
		pcToBundle = append(pcToBundle, loopIdx)
	}

	// Schedule bb2
	bb2MinIdx := len(bundles)
	bb2StartIdx := len(deps.bb0) + len(deps.bb1)
	for bb2Idx, dep := range deps.bb2 {
		// Check deps.
		minDepIdx := bb2MinIdx
		// Check bb0 deps
		for _, depPc := range dep.loopInvariantDeps {
			minDepIdx = maxInt(minDepIdx, getDepEarliestTime(depPc))
		}
		// Check bb1 deps
		for _, depPc := range dep.postLoopDeps {
			minDepIdx = maxInt(minDepIdx, getDepEarliestTime(depPc))
		}
		// Check bb2 deps
		for _, depPc := range dep.localDeps {
			minDepIdx = maxInt(minDepIdx, getDepEarliestTime(depPc))
		}

		i := bb2Idx + bb2StartIdx
		sI := &specIns{
			pred: nil,
			ins:  instrs[i],
		}

		// Find place for the op.
		idx := minDepIdx
		for {
			// Extend slice if needed.
			for len(bundles) <= idx {
				bundles = append(bundles, bundle{})
			}

			if bundles[idx].addInst(sI) {
				pcToBundle = append(pcToBundle, idx)
				break
			}
			idx++
		}
	}

	return
}

func (s *Scheduler) loopAllocateRegister(bundles []bundle, deps sectionDeps, pcToBundle []int) []bundle {
	currRegNum := uint8(1)
	instrs := make([]*instruction, len(pcToBundle))

	// Find loop instr
	var loopBundle int
	for i, b := range bundles {
		if b.branch != nil {
			loopBundle = i
			break
		}
	}

	// Phase 1
	for _, b := range bundles {
		for _, sI := range b.allSpecInstr() {
			if sI != nil {
				instr := &sI.ins
				instrs[instr.pc] = instr
				dst, _ := instr.mutRegs()
				if dst != nil && dst.type_ == xReg {
					dst.num = currRegNum
					currRegNum++
				}
			}
		}
	}

	// Phase 2
	allDeps := make([]dependency, 0, len(deps.bb0)+len(deps.bb1)+len(deps.bb2))
	allDeps = append(allDeps, deps.bb0...)
	allDeps = append(allDeps, deps.bb1...)
	allDeps = append(allDeps, deps.bb2...)

	for i, dep := range allDeps {
		instr := instrs[i]

		currDeps := make(map[reg]int, len(dep.loopInvariantDeps)+len(dep.localDeps)+len(dep.interloopDeps)+len(dep.postLoopDeps))
		for r, depPc := range dep.loopInvariantDeps {
			currDeps[r] = depPc
		}
		for r, depPc := range dep.localDeps {
			currDeps[r] = depPc
		}
		for r, iDep := range dep.interloopDeps {
			currDeps[r] = iDep.init
		}
		for r, depPc := range dep.postLoopDeps {
			currDeps[r] = depPc
		}

		_, ops := instr.mutRegs()
		for r, depPc := range currDeps {
			for _, op := range ops {
				if *op == r {
					dst, _ := instrs[depPc].regs()
					*op = *dst
				}
			}
		}
	}

	// Phase 3
	// TODO: this can be optimized by better choosing the order of the fixups.
	handled := make(map[reg]bool)
	for _, dep := range allDeps {
		for _, iDep := range dep.interloopDeps {
			bb1DepPc := iDep.body
			bb1DepInstr := instrs[bb1DepPc]
			bb1Dst, _ := bb1DepInstr.regs()

			bb0DepPc := iDep.init
			bb0DepInstr := instrs[bb0DepPc]
			bb0Dst, _ := bb0DepInstr.regs()

			if handled[*bb0Dst] {
				continue
			} else {
				handled[*bb0Dst] = true
			}

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

			fixUpBundleIdx := pcToBundle[bb1DepPc] + bb1DepInstr.latency()

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

	// Phase 4
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

	return bundles
}
