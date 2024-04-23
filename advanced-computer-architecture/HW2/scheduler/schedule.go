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
	pred *reg
	ins  instruction
}

type bundle struct {
	alu1, alu2, mult, mem, branch *specIns
}

func (b *bundle) allSpecInstr() []*specIns {
	return []*specIns{b.alu1, b.alu2, b.mult, b.mem, b.branch}
}

func (b *bundle) empty() bool {
	return b.alu1 == nil && b.alu2 == nil && b.mult == nil && b.mem == nil && b.branch == nil
}

type Scheduler struct {
}

type dependency struct {
	// Dependent register to instruction.
	localDeps         map[reg]int
	interloopDeps     map[reg]struct{ init, body int }
	loopInvariantDeps map[reg]int
	postLoopDeps      map[reg]int
}

type sectionDeps struct {
	bb0 []dependency
	bb1 []dependency
	bb2 []dependency
}

func newDependency() dependency {
	return dependency{
		localDeps:         make(map[reg]int),
		interloopDeps:     make(map[reg]struct{ init, body int }),
		loopInvariantDeps: make(map[reg]int),
		postLoopDeps:      make(map[reg]int),
	}
}

func (s *Scheduler) getDependencies(b blocks) (deps sectionDeps) {
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

		deps.bb0 = append(deps.bb0, dep)
	}

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

		deps.bb1 = append(deps.bb1, dep)
	}

	// Remove keys overwritten immediately by bb1.
	for key := range bb0Regs {
		if !bb0RegsUsed[key] {
			delete(bb0Regs, key)
		}
	}

	// Fill rest of bb1 keys.
	for i, instr := range b.bb1 {
		dep := &deps.bb1[i]
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

		deps.bb2 = append(deps.bb2, dep)
	}

	return deps
}

// FIXME:
// Multiple chains of instructions with data dependency, different instruction type, within the loop basic block.
// Loop with a single invariant dependency.
// Loop with multiple invariant dependencies.
// Loop with bubble in the basic block before the loop.

func (s *Scheduler) Schedule(instructions []string, outputLoop io.Writer, outputLoopPip io.Writer) error {
	outJsonLoop := json.NewEncoder(outputLoop)
	outJsonLoopPip := json.NewEncoder(outputLoopPip)

	ins, err := parseInstructions(instructions)
	if err != nil {
		return fmt.Errorf("error scheduling, %w", err)
	}

	// Temporary debug print of decoded instructions
	fmt.Println("Instructions:")
	for i, in := range ins {
		var r *reg
		if i == 0 {
			r = &reg{type_: predReg, num: 69}
		}
		m, err := json.Marshal(specIns{r, in})
		if err != nil {
			return err
		}
		fmt.Printf("%d: (%s)[%s]%#v\n", i, instructions[i], in, string(m))
	}

	blocks := splitIntoBlocks(ins)
	deps := s.getDependencies(blocks)

	// Temporary debug print of detected deps.
	fmt.Println()
	fmt.Println("Dependencies:")
	for i, dep := range deps.bb0 {
		fmt.Printf("(%d)[%c] %v\n", i, 'A'+i, dep)
	}
	for i, dep := range deps.bb1 {
		i += len(deps.bb0)
		fmt.Printf("(%d)[%c] %v\n", i, 'A'+i, dep)
	}
	for i, dep := range deps.bb2 {
		i += len(deps.bb0) + len(deps.bb1)
		fmt.Printf("(%d)[%c] %v\n", i, 'A'+i, dep)
	}

	// Loop
	loopSchedule, pcToBundle := s.loopSchedule(ins, deps)
	//if err := outJsonLoop.Encode(loopSchedule); err != nil {
	//	return fmt.Errorf("error scheduling, %w", err)
	//}
	loopAllocatedRegisters := s.loopAllocateRegister(loopSchedule, deps, pcToBundle)
	if err := outJsonLoop.Encode(loopAllocatedRegisters); err != nil {
		return fmt.Errorf("error scheduling, %w", err)
	}

	_ = outJsonLoopPip
	//panic("not implemented")
	return nil
}

func New() *Scheduler {
	return &Scheduler{}
}
