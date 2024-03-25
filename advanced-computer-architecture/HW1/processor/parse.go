package processor

import (
	"fmt"
	"strconv"
	"strings"
)

func parseMnemonic(op string) (InstructionType, error) {
	i := InstructionType(op)
	for _, menomic := range allInstructions {
		if i == menomic {
			return i, nil
		}
	}
	return "", fmt.Errorf("unknown mnemonic: %s", op)
}

func parseReg(op string) (LogicReg, error) {
	regNum, err := strconv.ParseUint(op[1:], 10, 8)
	if op[0] != 'x' || err != nil || regNum > 31 {
		return 0, fmt.Errorf("invalid register: %s", op)
	}
	return LogicReg(regNum), nil
}

func parseImm(op string) (int64, error) {
	regNum, err := strconv.ParseInt(op, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("marformed immediate")
	}
	return regNum, nil
}

func parseInstruction(asm string) (ins instruction, err error) {
	var parts []string

	split := strings.Fields(asm)
	if len(split) != 0 {
		parts = append(parts, split[0])
		operands := strings.Split(strings.Join(split[1:], ""), ",")
		parts = append(parts, operands...)
	}
	if len(parts) != 4 {
		return ins, fmt.Errorf("malformed instruction: %s", asm)
	}

	ins.type_, err = parseMnemonic(parts[0])
	if err != nil {
		return ins, err
	}

	ins.dest, err = parseReg(parts[1])
	if err != nil {
		return ins, err
	}

	ins.opA, err = parseReg(parts[2])
	if err != nil {
		return ins, err
	}

	if ins.type_ == addi {
		ins.opB.imm, err = parseImm(parts[3])
	} else {
		ins.opB.reg, err = parseReg(parts[3])
	}
	if err != nil {
		return ins, err
	}

	return ins, nil
}
