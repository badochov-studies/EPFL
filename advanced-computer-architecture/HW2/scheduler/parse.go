package scheduler

import (
	"fmt"
	"strconv"
	"strings"
)

func parseMnemonic(op string) (instructionType, error) {
	i := instructionType(op)
	for _, menomic := range allInstructions {
		if i == menomic {
			return i, nil
		}
	}
	return "", fmt.Errorf("unknown mnemonic: %s", op)
}

func parseReg(op string) (reg, error) {
	var res reg

	switch op {
	case "LC":
		res.type_ = specialReg
		res.num = lcReg
		return res, nil
	case "EC":
		res.type_ = specialReg
		res.num = ecReg
		return res, nil
	}

	switch op[0] {
	case 'x':
		res.type_ = aluReg
	case 'p':
		res.type_ = predReg
	default:
		return reg{}, fmt.Errorf("invalid register: %s", op)
	}
	regNum, err := strconv.ParseUint(op[1:], 10, 8)
	if err != nil || regNum > 95 {
		return reg{}, fmt.Errorf("invalid register: %s", op)
	}
	res.num = uint8(regNum)
	return res, nil
}

func parseAluReg(op string) (reg, error) {
	res, err := parseReg(op)
	if err != nil {
		return reg{}, fmt.Errorf("error parsing an alu reg, %w", err)
	}
	if res.type_ != aluReg {
		return reg{}, fmt.Errorf("not an alu reg '%s'", op)
	}
	return res, nil
}

func parseImm(op string) (int64, error) {
	regNum, err := strconv.ParseInt(op, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("marformed immediate")
	}
	return regNum, nil
}

func parseAddr(op string) (reg, int64, error) {
	addrSplit := strings.Split(op, "(")
	if len(addrSplit) != 2 || len(addrSplit[1]) == 0 || addrSplit[1][len(addrSplit[1])-1] != ')' {
		return reg{}, 0, fmt.Errorf("malformed address: %s", op)
	}
	reg_, err := parseAluReg(addrSplit[1][:len(addrSplit[1])-1])
	if err != nil {
		fmt.Println(addrSplit[1], addrSplit[1][:len(addrSplit[1])-1])
		return reg{}, 0, fmt.Errorf("malformed address: %s, %v", op, err)
	}
	imm, err := parseImm(addrSplit[0])
	if err != nil {
		return reg{}, 0, fmt.Errorf("malformed address %s, %v", op, err)
	}
	return reg_, imm, nil
}

func parseInstruction(asm string) (ins instruction, err error) {
	var parts []string

	split := strings.Fields(asm)
	if len(split) != 0 {
		parts = append(parts, split[0])
		operands := strings.Split(strings.Join(split[1:], ""), ",")
		parts = append(parts, operands...)
	}
	if len(parts) == 0 {
		return ins, fmt.Errorf("malformed instruction: %s", asm)
	}

	ins.type_, err = parseMnemonic(parts[0])
	if err != nil {
		return
	}
	switch ins.type_ {
	case add, sub, mulu, addi:
		if len(parts) != 4 {
			return ins, fmt.Errorf("malformed instruction: %s", asm)
		}
		ins.regA, err = parseAluReg(parts[1])
		if err != nil {
			return
		}

		ins.regB, err = parseAluReg(parts[2])
		if err != nil {
			return
		}

		if ins.type_ == addi {
			ins.imm, err = parseImm(parts[3])
		} else {
			ins.regC, err = parseAluReg(parts[3])
		}
	case ld, st:
		if len(parts) != 3 {
			return ins, fmt.Errorf("malformed instruction: %s", asm)
		}
		ins.regA, err = parseAluReg(parts[1])
		if err != nil {
			return
		}

		ins.regB, ins.imm, err = parseAddr(parts[2])
	case loop, loopPip:
		if len(parts) != 2 {
			return ins, fmt.Errorf("malformed instruction: %s", asm)
		}
		ins.imm, err = parseImm(parts[1])
	case mov:
		if len(parts) != 3 {
			return ins, fmt.Errorf("malformed instruction: %s", asm)
		}
		ins.regA, err = parseReg(parts[1])
		if err != nil {
			return
		}

		switch ins.regA.type_ {
		case aluReg:
			ins.regB, err = parseAluReg(parts[2])
			if err != nil {
				ins.imm, err = parseImm(parts[2])
				if err != nil {
					return ins, fmt.Errorf("malformed instruction: %s", asm)
				}
			} else {
				ins.usesReg = true
			}
		case predReg:
			switch parts[2] {
			case "true":
				ins.pred = true
			case "false":
				ins.pred = false
			default:
				return ins, fmt.Errorf("malformed instruction: %s", asm)
			}
		case specialReg:
			ins.imm, err = parseImm(parts[2])
		}
	case nop:
		if len(parts) != 1 {
			return ins, fmt.Errorf("malformed instruction: %s", asm)
		}
	default:
		panic("impossible!")
	}

	return
}

func parseInstructions(instructions []string) (res []instruction, err error) {
	res = make([]instruction, len(instructions))
	for i, ins := range instructions {
		res[i], err = parseInstruction(ins)
		if err != nil {
			err = fmt.Errorf("error parsing instruction %d, %w", i, err)
			break
		}
		res[i].pc = i
	}
	return
}
