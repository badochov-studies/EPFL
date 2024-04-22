package scheduler

import (
	"encoding/json"
	"fmt"
)

func (i instruction) String() string {
	switch i.type_ {
	case add, sub, mulu:
		return fmt.Sprintf("%s %s, %s, %s", i.type_, i.regA, i.regB, i.regC)
	case addi:
		return fmt.Sprintf("%s %s, %s, %d", i.type_, i.regA, i.regB, i.imm)
	case ld, st:
		return fmt.Sprintf("%s %s, %d(%s)", i.type_, i.regA, i.imm, i.regB)
	case loop, loopPip:
		return fmt.Sprintf("%s %d", i.type_, i.imm)
	case mov:
		switch i.regA.type_ {
		case aluReg:
			if i.usesReg {
				return fmt.Sprintf("%s %s, %s", i.type_, i.regA, i.regB)
			} else {
				return fmt.Sprintf("%s %s, %d", i.type_, i.regA, i.imm)
			}
		case predReg:
			return fmt.Sprintf("%s %s, %t", i.type_, i.regA, i.pred)
		case specialReg:
			return fmt.Sprintf("%s %s, %d", i.type_, i.regA, i.imm)
		default:
			panic("impossible!")
		}
	case nop:
		return string(i.type_)
	default:
		panic("impossible!")
	}
}

func (r reg) String() string {
	switch r.type_ {
	case aluReg:
		return fmt.Sprintf("x%d", r.num)
	case predReg:
		return fmt.Sprintf("p%d", r.num)
	case specialReg:
		switch r.num {
		case ecReg:
			return "EC"
		case lcReg:
			return "LC"
		default:
			panic("impossible!")
		}
	default:
		panic("impossible!")
	}
}

func (s specIns) MarshalText() ([]byte, error) {
	res := ""
	if s.pred != nil {
		res = fmt.Sprintf("(%s) ", *s.pred)
	}

	return []byte(res + s.ins.String()), nil
}

func (b bundle) MarshalJSON() ([]byte, error) {
	return json.Marshal([]specIns{b.alu1, b.alu2, b.mult, b.mem, b.branch})
}
