package scheduler

import (
	"encoding/json"
	"fmt"
	"io"
)

type instructionType string

const (
	add     instructionType = "add"
	addi    instructionType = "addi"
	sub     instructionType = "sub"
	mulu    instructionType = "mulu"
	ld      instructionType = "ld"
	st      instructionType = "st"
	loop    instructionType = "loop"
	loopPip instructionType = "loop.pip"
	nop     instructionType = "nop"
	mov     instructionType = "mov"
)

var allInstructions = []instructionType{add, addi, sub, mulu, ld, st, loop, loopPip, nop, mov}

type regType uint8

const (
	xReg regType = iota
	predReg
	specialReg
)

const (
	lcReg uint8 = iota
	ecReg
)

type reg struct {
	type_ regType
	num   uint8
}

type instruction struct {
	pc      int
	type_   instructionType
	regA    reg
	regB    reg
	regC    reg
	imm     int64
	pred    bool
	usesReg bool
}

type specIns struct {
	pred  *reg
	instr instruction
}

type Scheduler struct {
}

type dependency struct {
	pc int
	// Dependent register to instruction.
	localDeps         map[reg]int
	interloopDeps     map[reg]struct{ init, body int }
	loopInvariantDeps map[reg]int
	postLoopDeps      map[reg]int
}

func (d dependency) nonInterloopBodyDeps() map[reg]int {
	deps := make(map[reg]int, len(d.loopInvariantDeps)+len(d.localDeps)+len(d.interloopDeps)+len(d.postLoopDeps))
	for r, depPc := range d.loopInvariantDeps {
		deps[r] = depPc
	}
	for r, depPc := range d.localDeps {
		deps[r] = depPc
	}
	for r, iDep := range d.interloopDeps {
		deps[r] = iDep.init
	}
	for r, depPc := range d.postLoopDeps {
		deps[r] = depPc
	}
	return deps
}

type sectionDeps struct {
	deps     []dependency
	bb1Start int
	bb2Start int
}

func (sd sectionDeps) bb0() []dependency {
	return sd.deps[:sd.bb1Start]
}

func (sd sectionDeps) bb1() []dependency {
	return sd.deps[sd.bb1Start:sd.bb2Start]
}

func (sd sectionDeps) bb2() []dependency {
	return sd.deps[sd.bb2Start:]
}

func newDependency() dependency {
	return dependency{
		localDeps:         make(map[reg]int),
		interloopDeps:     make(map[reg]struct{ init, body int }),
		loopInvariantDeps: make(map[reg]int),
		postLoopDeps:      make(map[reg]int),
	}
}

func (s *Scheduler) getDependencies(b blockInstrs) (deps sectionDeps) {
	// Find deps in bb0.
	bb0Regs := make(map[reg]int)

	for _, instr := range b.bb0 {
		dep := newDependency()
		dst, ops := instr.regs()

		for _, op := range ops {
			if opDep, ok := bb0Regs[op]; ok {
				dep.localDeps[op] = opDep
			} else {
				// Operand not used previously.
			}
		}

		if dst != nil {
			bb0Regs[*dst] = instr.pc
		}

		dep.pc = len(deps.deps)
		deps.deps = append(deps.deps, dep)
	}
	deps.bb1Start = len(deps.deps)

	// Find non-local deps in bb1.
	bb1Regs := make(map[reg]int)
	bb0RegsUsed := make(map[reg]bool)
	for _, instr := range b.bb1 {
		dep := newDependency()
		dst, ops := instr.regs()

		for _, op := range ops {
			if bb1Dep, ok := bb1Regs[op]; ok {
				dep.localDeps[op] = bb1Dep
			} else {
				bb0RegsUsed[op] = true
			}
		}

		if dst != nil {
			bb1Regs[*dst] = instr.pc
		}

		dep.pc = len(deps.deps)
		deps.deps = append(deps.deps, dep)
	}

	// Remove keys overwritten immediately by bb1.
	for key := range bb0Regs {
		if !bb0RegsUsed[key] {
			delete(bb0Regs, key)
		}
	}

	// Fill rest of bb1 keys.
	for _, instr := range b.bb1 {
		dep := &deps.deps[instr.pc]
		_, ops := instr.regs()

		for _, op := range ops {
			bb0Dep, fromBB0 := bb0Regs[op]
			bb1Dep, fromBB1 := bb1Regs[op]

			if fromBB0 && fromBB1 {
				dep.interloopDeps[op] = struct{ init, body int }{bb0Dep, bb1Dep}
			} else if fromBB0 {
				dep.loopInvariantDeps[op] = bb0Dep
			} else if fromBB1 {
				// Assigned in previous loop pass.
			} else {
				// Operand not used previously.
			}
		}
	}
	deps.bb2Start = len(deps.deps)

	// Add bb2 deps.
	bb2Regs := make(map[reg]int)
	for _, instr := range b.bb2 {
		dep := newDependency()
		dst, ops := instr.regs()

		for _, op := range ops {
			if bb2Dep, fromBB2 := bb2Regs[op]; fromBB2 {
				dep.localDeps[op] = bb2Dep
			} else if bb1Dep, fromBB1 := bb1Regs[op]; fromBB1 {
				dep.postLoopDeps[op] = bb1Dep
			} else if bb0Dep, fromBB0 := bb0Regs[op]; fromBB0 {
				dep.loopInvariantDeps[op] = bb0Dep
			} else {
				// Operand not used previously.
			}
		}

		if dst != nil {
			bb2Regs[*dst] = instr.pc
		}

		dep.pc = len(deps.deps)
		deps.deps = append(deps.deps, dep)
	}

	return deps
}

func (s *Scheduler) Schedule(instructions []string, outputLoop io.Writer, outputLoopPip io.Writer) error {
	outJsonLoop := json.NewEncoder(outputLoop)
	outJsonLoopPip := json.NewEncoder(outputLoopPip)

	instrs, err := parseInstructions(instructions)
	if err != nil {
		return fmt.Errorf("error scheduling, %w", err)
	}

	// Temporary debug print of decoded instructions
	fmt.Println("Instructions:")
	for i, instr := range instrs {
		var r *reg
		if i == 0 {
			r = &reg{type_: predReg, num: 69}
		}
		m, err := json.Marshal(specIns{r, instr})
		if err != nil {
			return err
		}
		fmt.Printf("%d: (%s)[%s]%#v\n", i, instructions[i], instr, string(m))
	}

	blocks := splitIntoBlocks(instrs)
	deps := s.getDependencies(blocks)

	// Temporary debug print of detected deps.
	fmt.Println()
	fmt.Println("Dependencies:")
	for i, dep := range deps.deps {
		fmt.Printf("(%d)[%c] %v\n", i, 'A'+i, dep)
	}

	// Loop
	ls := newLoopScheduler(instrs, deps)

	loopBundles := ls.schedule()
	if err = outJsonLoop.Encode(loopBundles); err != nil {
		return fmt.Errorf("error scheduling, %w", err)
	}

	// LoopPip
	lps := newLoopPipScheduler(instrs, deps)

	loopPipBundles := lps.schedule()
	if err = outJsonLoopPip.Encode(loopPipBundles); err != nil {
		return fmt.Errorf("error scheduling, %w", err)
	}
	_ = outJsonLoopPip

	return nil
}

func New() *Scheduler {
	return &Scheduler{}
}
