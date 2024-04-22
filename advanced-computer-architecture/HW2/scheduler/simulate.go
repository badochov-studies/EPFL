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
	aluReg regType = iota
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
	alu1, alu2, mult, mem, branch specIns
}

type Scheduler struct {
}

func (s *Scheduler) Simulate(instructions []string, outputLoop io.Writer, outputLoopPip io.Writer) error {
	ins, err := parseInstructions(instructions)
	if err != nil {
		return fmt.Errorf("error simulating, %w", err)
	}
	// Temporary debug print of decoded instructions
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

	outJsonLoop := json.NewEncoder(outputLoop)
	outJsonLoopPip := json.NewEncoder(outputLoopPip)

	_, _ = outJsonLoop, outJsonLoopPip
	panic("not implemented")
}

func New() *Scheduler {
	return &Scheduler{}
}
