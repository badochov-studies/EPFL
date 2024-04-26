package scheduler

import (
	"fmt"
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

type blockInstrs struct {
	bb0 []instruction
	bb1 []instruction
	bb2 []instruction
}

func splitIntoBlocks(instrs []instruction) blockInstrs {
	for i, instr := range instrs {
		if instr.type_.isBranch() {
			to := instr.imm
			return blockInstrs{
				bb0: instrs[:to],
				bb1: instrs[to : i+1],
				bb2: instrs[i+1:],
			}
		}
	}

	return blockInstrs{
		bb0: instrs,
	}
}

func (i instruction) latency() int {
	if i.type_.isMul() {
		return 3
	}
	return 1
}

func maxInt(max int, nums ...int) int {
	for _, num := range nums {
		if num > max {
			max = num
		}
	}
	return max
}

type bundleSlot int8

const noSlot bundleSlot = -1
const (
	alu1 bundleSlot = iota
	alu2
	mult
	mem
	branch
)

type bundle [5]*specIns

func (b *bundle) addInst(sI *specIns) bundleSlot {
	if sI.instr.type_.isMul() {
		if b[mult] == nil {
			b[mult] = sI
			return mult
		}
	} else if sI.instr.type_.isAlu() {
		if b[alu1] == nil {
			b[alu1] = sI
			return alu1
		}
		if b[alu2] == nil {
			b[alu2] = sI
			return alu2
		}
	} else if sI.instr.type_.isMem() {
		if b[mem] == nil {
			b[mem] = sI
			return mem
		}
	} else {
		panic(fmt.Sprint("Unexpected instruction type to add:", sI.instr.type_))
	}
	return noSlot
}

func (b *bundle) empty() bool {
	for _, slot := range b {
		if slot != nil && slot.instr.type_ != nop {
			return false
		}
	}
	return true
}

func (b *bundle) vTakeSlot(slot bundleSlot) {
	if b[slot] == nil {
		b[slot] = &specIns{
			pred: nil,
			instr: instruction{
				pc:    -1,
				type_: nop,
			},
		}
	}
}

type blockBundles struct {
	bb0 []bundle
	bb1 []bundle
	bb2 []bundle
}

func (bb *blockBundles) bb0Start() int {
	return 0
}

func (bb *blockBundles) bb1Start() int {
	return bb.bb0Start() + len(bb.bb0)
}

func (bb *blockBundles) bb2Start() int {
	return bb.bb1Start() + len(bb.bb1)
}

func (bb *blockBundles) len() int {
	return len(bb.bb0) + len(bb.bb1) + len(bb.bb2)
}

func (bb *blockBundles) get(idx int) *bundle {
	if idx < len(bb.bb0) {
		return &bb.bb0[idx]
	}
	idx -= len(bb.bb0)
	if idx < len(bb.bb1) {
		return &bb.bb1[idx]
	}
	idx -= len(bb.bb1)
	if idx < len(bb.bb2) {
		return &bb.bb2[idx]
	}
	panic("Tried to access unreachable bundles idx!")
}

func (bb *blockBundles) getBlockByStartIdx(bbStartIdx int) *[]bundle {
	switch bbStartIdx {
	case bb.bb0Start():
		return &bb.bb0
	case bb.bb1Start():
		return &bb.bb1
	case bb.bb2Start():
		return &bb.bb2
	default:
		panic("Invalid bbStartIdx")
	}
}

func (bb *blockBundles) extend(bbStartIdx int, length int) {
	bb.extendBlockBy(bbStartIdx, length-bb.len())
}

func (bb *blockBundles) extendBlockBy(bbStartIdx int, by int) {
	block := bb.getBlockByStartIdx(bbStartIdx)
	for i := 0; i < by; i++ {
		*block = append(*block, bundle{})
	}
}

func (bb *blockBundles) trimStart(bbStartIdx int, by int) {
	block := bb.getBlockByStartIdx(bbStartIdx)
	copy(*block, (*block)[by:])
}

func (bb *blockBundles) shrinkBlock(bbStartIdx int, blockLength int) {
	block := bb.getBlockByStartIdx(bbStartIdx)
	*block = (*block)[:blockLength]
}

func (bb *blockBundles) bb0AndBB2() []bundle {
	res := make([]bundle, len(bb.bb0)+len(bb.bb2))
	copy(res, bb.bb0)
	copy(res[len(bb.bb0):], bb.bb2)

	return res
}

func getLoopBundleIdx(bb *blockBundles) int {
	for i := 0; i < bb.len(); i++ {
		if bb.get(i)[branch] != nil {
			return i
		}
	}
	return -1
}

func loopStages(bb *blockBundles, II int) int {
	return (len(bb.bb1) + II - 1) / II
}
