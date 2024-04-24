package scheduler

import (
	"fmt"
	"math"
)

func (i instruction) regs() (dst *reg, params []reg) {
	regA := i.regA
	switch i.type_ {
	case add, sub, mulu:
		return &regA, []reg{i.regB, i.regC}
	case addi,
		ld:
		return &regA, []reg{i.regB}
	case st:
		return nil, []reg{regA, i.regB}
	case mov:
		if regA.type_ == xReg && i.usesReg {
			return &regA, []reg{i.regB}
		}
		return &regA, nil
	case loop, loopPip, nop:
		return nil, nil
	default:
		panic("Impossible")
	}
}

func (i *instruction) mutRegs() (dst *reg, params []*reg) {
	switch i.type_ {
	case add, sub, mulu:
		return &i.regA, []*reg{&i.regB, &i.regC}
	case addi,
		ld:
		return &i.regA, []*reg{&i.regB}
	case st:
		return nil, []*reg{&i.regA, &i.regB}
	case mov:
		if i.regA.type_ == xReg && i.usesReg {
			return &i.regA, []*reg{&i.regB}
		}
		return &i.regA, nil
	case loop, loopPip, nop:
		return nil, nil
	default:
		panic("Impossible")
	}
}

func (it instructionType) isAlu() bool {
	switch it {
	case add, addi, sub, mov:
		return true
	default:
		return false
	}
}

func (it instructionType) isMul() bool {
	switch it {
	case mulu:
		return true
	default:
		return false
	}
}

func (it instructionType) isMem() bool {
	switch it {
	case st, ld:
		return true
	default:
		return false
	}
}

func (it instructionType) isBranch() bool {
	switch it {
	case loop, loopPip:
		return true
	default:
		return false
	}
}

func (b *bundle) addInst(sI *specIns) bundleSlot {
	if sI.ins.type_.isMul() {
		if b[mult] == nil {
			b[mult] = sI
			return mult
		}
	} else if sI.ins.type_.isAlu() {
		if b[alu1] == nil {
			b[alu1] = sI
			return alu1
		}
		if b[alu2] == nil {
			b[alu2] = sI
			return alu2
		}
	} else if sI.ins.type_.isMem() {
		if b[mem] == nil {
			b[mem] = sI
			return mem
		}
	} else {
		panic(fmt.Sprint("Unexpected instruction type to add:", sI.ins.type_))
	}
	return noSlot
}

type blocks struct {
	bb0 []instruction
	bb1 []instruction
	bb2 []instruction
}

func splitIntoBlocks(instrs []instruction) blocks {
	for i, instr := range instrs {
		if instr.type_.isBranch() {
			to := instr.imm
			return blocks{
				bb0: instrs[:to],
				bb1: instrs[to : i+1],
				bb2: instrs[i+1:],
			}
		}
	}

	return blocks{
		bb0: instrs,
	}
}

func (i instruction) latency() int {
	if i.type_.isMul() {
		return 3
	}
	return 1
}

func maxInt(nums ...int) int {
	max := math.MinInt
	for _, num := range nums {
		if num > max {
			max = num
		}
	}
	return max
}

func getLoopBundle(bundles []bundle) int {
	for i, b := range bundles {
		if b[branch] != nil {
			return i
		}
	}
	return -1
}
