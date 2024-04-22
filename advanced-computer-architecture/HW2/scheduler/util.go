package scheduler

func (i instruction) regs() (dst *reg, params []reg) {
	regA := i.regA
	switch i.type_ {
	case add, sub, mulu:
		return &regA, []reg{i.regB, i.regC}
	case addi,
		ld, st:
		return &regA, []reg{i.regB}
	case mov:
		if regA.type_ == aluReg && i.usesReg {
			return &regA, []reg{i.regB}
		}
		return &regA, nil
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

type block struct {
	instructions []instruction
	isLoop       bool
}

func (b block) startPC() int {
	if len(b.instructions) == 0 {
		return -1
	}
	return b.instructions[0].pc
}

func splitIntoBlock(instructions []instruction) (res []block) {
	blockStart := 0

	for i, ins := range instructions {
		if ins.type_.isBranch() {
			to := ins.imm
			initBlock := block{
				instructions: instructions[blockStart:to],
				isLoop:       false,
			}
			loopBlock := block{
				instructions: instructions[to : i+1],
				isLoop:       true,
			}
			blockStart = i + 1
			res = append(res, initBlock, loopBlock)
		}
	}

	finishBlock := block{
		instructions: instructions[blockStart:],
		isLoop:       false,
	}
	res = append(res, finishBlock)

	return
}

func (i instruction) latency() int {
	if i.type_.isMul() {
		return 3
	}
	return 1
}
