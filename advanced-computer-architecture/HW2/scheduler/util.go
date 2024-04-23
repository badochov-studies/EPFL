package scheduler

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

type blocks struct {
	bb0 []instruction
	bb1 []instruction
	bb2 []instruction
}

func splitIntoBlocks(instructions []instruction) blocks {
	for i, ins := range instructions {
		if ins.type_.isBranch() {
			to := ins.imm
			return blocks{
				bb0: instructions[:to],
				bb1: instructions[to : i+1],
				bb2: instructions[i+1:],
			}
		}
	}

	return blocks{
		bb0: instructions,
	}
}

func (i instruction) latency() int {
	if i.type_.isMul() {
		return 3
	}
	return 1
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
