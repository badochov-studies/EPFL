package processor

import (
	"encoding/json"
	"io"
)

type InstructionType string

type LogicReg int8
type PhysReg int8

const (
	add  InstructionType = "add"
	addi InstructionType = "addi"
	sub  InstructionType = "sub"
	mulu InstructionType = "mulu"
	divu InstructionType = "divu"
	remu InstructionType = "remu"
)

func (i InstructionType) toOpCode() string {
	if i == addi {
		return "add"
	}
	return string(i)
}

var allInstructions = []InstructionType{add, addi, sub, mulu, divu, remu}

type instruction struct {
	type_ InstructionType
	dest  LogicReg
	opA   LogicReg
	opB   struct {
		reg LogicReg
		imm int64
	}
}

type activeListEntry struct {
	Done               bool
	Exception          bool
	LogicalDestination LogicReg
	OldDestination     PhysReg
	PC                 uint64
}

type integerQueueEntry struct {
	DestRegister PhysReg
	OpAIsReady   bool
	OpARegTag    PhysReg
	OpAValue     uint64
	OpBIsReady   bool
	OpBRegTag    PhysReg
	OpBValue     uint64
	OpCode       string
	PC           uint64
}

func (e integerQueueEntry) ready() bool {
	return e.OpAIsReady && e.OpBIsReady
}

type freeList []PhysReg

func (f *freeList) hasEnoughFreeEntries(n int) bool {
	return len(*f)-n >= 0
}

func (f *freeList) get(n int) []PhysReg {
	res := (*f)[:n]
	*f = (*f)[n:]
	return res
}

type activeList []activeListEntry

func (a *activeList) hasEnoughFreeEntries(n int) bool {
	return len(*a)+n < 32
}

func (a *activeList) getEntryByPC(pc uint64) *activeListEntry {
	for i := range *a {
		ale := &(*a)[i]
		if ale.PC == pc {
			return ale
		}
	}
	return nil
}

func (a *activeList) headDone() bool {
	return len(*a) != 0 && (*a)[0].Done
}

func (a *activeList) top() activeListEntry {
	return (*a)[0]
}

func (a *activeList) pop() {
	*a = (*a)[1:]
}

type integerQueue []integerQueueEntry

func (i *integerQueue) hasEnoughFreeEntries(n int) bool {
	return len(*i)+n < 32
}

func (i *integerQueue) take(pos int) *integerQueueEntry {
	res := (*i)[pos]
	*i = append((*i)[:pos], (*i)[pos+1:]...)
	return &res
}

type alu struct {
	assigned   *integerQueueEntry
	inProgress *integerQueueEntry
	done       *integerQueueEntry
}

func (a *alu) ready() *integerQueueEntry {
	return a.done
}

func (a *alu) result() (uint64, bool) {
	switch a.done.OpCode {
	case add.toOpCode(), addi.toOpCode():
		return i64Tou64(u64Toi64(a.done.OpAValue) + u64Toi64(a.done.OpBValue)), false
	case sub.toOpCode():
		return i64Tou64(u64Toi64(a.done.OpAValue) - u64Toi64(a.done.OpBValue)), false
	case mulu.toOpCode():
		return a.done.OpAValue * a.done.OpBValue, false
	case divu.toOpCode():
		if a.done.OpBValue == 0 {
			return 0, true
		}
		return a.done.OpAValue / a.done.OpBValue, false
	case remu.toOpCode():
		if a.done.OpBValue == 0 {
			return 0, true
		}
		return a.done.OpAValue % a.done.OpBValue, false
	default:
		panic("unexpected opCode: " + a.done.OpCode)
	}
}

func (a *alu) progress() {
	a.done = a.inProgress
	a.inProgress = a.assigned
	a.assigned = nil
}

func (a *alu) assign(entry *integerQueueEntry) {
	a.assigned = entry
}

type state struct {
	PC uint64

	PhysicalRegisterFile [64]uint64

	DecodedPCs []uint64

	ExceptionPC uint64
	Exception   bool

	RegisterMapTable [32]PhysReg

	FreeList freeList

	BusyBitTable [64]bool

	ActiveList activeList

	IntegerQueue integerQueue

	backpressure bool

	alu [4]alu
}

type Processor struct {
	state        state
	workingState state
	instructions []instruction
	log          []state
	end          bool
}

func (s *state) copy() state {
	copied := *s

	copied.ActiveList = make(activeList, len(s.ActiveList))
	copy(copied.ActiveList, s.ActiveList)
	copied.IntegerQueue = make(integerQueue, len(s.IntegerQueue))
	copy(copied.IntegerQueue, s.IntegerQueue)
	copied.FreeList = make(freeList, len(s.FreeList))
	copy(copied.FreeList, s.FreeList)
	copied.DecodedPCs = make([]uint64, len(s.DecodedPCs))
	copy(copied.DecodedPCs, s.DecodedPCs)

	return copied
}

func (p *Processor) parseInstructions(instructions []string) (err error) {
	p.instructions = make([]instruction, len(instructions))
	for i, ins := range instructions {
		p.instructions[i], err = parseInstruction(ins)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) dumpStateIntoLog() {
	s := p.state.copy()
	// Gotta to hate go's default json package for that.
	if s.ActiveList == nil {
		s.ActiveList = make(activeList, 0)
	}
	if s.IntegerQueue == nil {
		s.IntegerQueue = make(integerQueue, 0)
	}
	if s.FreeList == nil {
		s.FreeList = make(freeList, 0)
	}
	if s.DecodedPCs == nil {
		s.DecodedPCs = make([]uint64, 0)
	}
	p.log = append(p.log, s)
}

func (p *Processor) fetchAndDecode() {
	if p.workingState.Exception {
		p.workingState.PC = 0x10000
		return
	}

	if p.workingState.backpressure {
		return
	}

	for i := 0; i < 4; i++ {
		if p.workingState.PC >= uint64(len(p.instructions)) {
			break
		}
		p.workingState.DecodedPCs = append(p.workingState.DecodedPCs, p.workingState.PC)
		p.workingState.PC++
	}
}

func (p *Processor) renameAndDispatch() {
	if p.workingState.Exception {
		p.workingState.DecodedPCs = nil
		return
	}

	numInstructions := len(p.workingState.DecodedPCs)

	p.workingState.backpressure = !(p.workingState.ActiveList.hasEnoughFreeEntries(numInstructions) &&
		p.workingState.IntegerQueue.hasEnoughFreeEntries(numInstructions) &&
		p.workingState.FreeList.hasEnoughFreeEntries(numInstructions))
	if p.workingState.backpressure {
		return
	}

	newDestRegs := p.workingState.FreeList.get(numInstructions)
	for i, insPc := range p.workingState.DecodedPCs {
		ins := p.instructions[insPc]

		iqe := integerQueueEntry{
			DestRegister: newDestRegs[i],
			OpAIsReady:   false,
			OpARegTag:    p.workingState.RegisterMapTable[ins.opA],
			OpAValue:     0,
			OpBIsReady:   false,
			OpBRegTag:    0,
			OpBValue:     0,
			OpCode:       ins.type_.toOpCode(),
			PC:           insPc,
		}
		if !p.workingState.BusyBitTable[iqe.OpARegTag] {
			iqe.OpAIsReady = true
			iqe.OpAValue = p.workingState.PhysicalRegisterFile[iqe.OpARegTag]
		}
		if ins.type_ == addi {
			iqe.OpBIsReady = true
			iqe.OpBValue = i64Tou64(ins.opB.imm)
		} else {
			iqe.OpBRegTag = p.workingState.RegisterMapTable[ins.opB.reg]
			if !p.workingState.BusyBitTable[iqe.OpBRegTag] {
				iqe.OpBIsReady = true
				iqe.OpBValue = p.workingState.PhysicalRegisterFile[iqe.OpBRegTag]
			}
		}

		p.workingState.IntegerQueue = append(p.workingState.IntegerQueue, iqe)

		p.workingState.ActiveList = append(p.workingState.ActiveList, activeListEntry{
			Done:               false,
			Exception:          false,
			LogicalDestination: ins.dest,
			OldDestination:     p.workingState.RegisterMapTable[ins.dest],
			PC:                 insPc,
		})

		p.workingState.RegisterMapTable[ins.dest] = newDestRegs[i]

		p.workingState.BusyBitTable[newDestRegs[i]] = true
	}

	p.workingState.DecodedPCs = nil
}

func (p *Processor) issue() {
	if p.workingState.Exception {
		p.workingState.IntegerQueue = nil
		return
	}
	for i := range p.workingState.alu {
		alu := &p.workingState.alu[i]
		for j := range p.workingState.IntegerQueue {
			iqe := &p.workingState.IntegerQueue[j]
			if iqe.ready() {
				alu.assign(p.workingState.IntegerQueue.take(j))
				break
			}
		}
	}
}

func (p *Processor) updateIntegerQueueReadiness() {
	for i := range p.workingState.IntegerQueue {
		iqe := &p.workingState.IntegerQueue[i]

		if !iqe.OpAIsReady && !p.workingState.BusyBitTable[iqe.OpARegTag] {
			iqe.OpAIsReady = true
			iqe.OpAValue = p.workingState.PhysicalRegisterFile[iqe.OpARegTag]
		}
		if !iqe.OpBIsReady && !p.workingState.BusyBitTable[iqe.OpBRegTag] {
			iqe.OpBIsReady = true
			iqe.OpBValue = p.workingState.PhysicalRegisterFile[iqe.OpBRegTag]
		}
	}
}

func (p *Processor) execute() {
	if p.workingState.Exception {
		return
	}
	for i := range p.workingState.alu {
		alu := &p.workingState.alu[i]
		alu.progress()
		entry := alu.ready()
		if entry != nil {
			res, exc := alu.result()

			ale := p.workingState.ActiveList.getEntryByPC(entry.PC)
			ale.Done = true

			if exc {
				ale.Exception = true
			} else {
				p.workingState.PhysicalRegisterFile[entry.DestRegister] = res
				p.workingState.BusyBitTable[entry.DestRegister] = false
			}
		}
	}

	p.updateIntegerQueueReadiness()
}

func (p *Processor) recover() {
	if len(p.workingState.ActiveList) == 0 {
		p.workingState.Exception = false
		p.end = true
	}

	for i := 0; i < 4 && len(p.workingState.ActiveList) != 0; i++ {
		ale := p.workingState.ActiveList[len(p.workingState.ActiveList)-1]
		p.workingState.ActiveList = p.workingState.ActiveList[:len(p.workingState.ActiveList)-1]

		prevReg := p.workingState.RegisterMapTable[ale.LogicalDestination]
		p.workingState.BusyBitTable[prevReg] = false
		p.workingState.RegisterMapTable[ale.LogicalDestination] = ale.OldDestination
		p.workingState.FreeList = append(p.workingState.FreeList, prevReg)
	}
}

func (p *Processor) commit() {
	if p.workingState.Exception {
		p.recover()
		return
	}

	for i := 0; i < 4 && p.workingState.ActiveList.headDone(); i++ {
		ale := p.workingState.ActiveList.top()
		if ale.Exception {
			p.workingState.Exception = true
			p.workingState.ExceptionPC = ale.PC
			break
		}
		p.workingState.FreeList = append(p.workingState.FreeList, ale.OldDestination)
		p.workingState.ActiveList.pop()
	}
}

func (p *Processor) propagate() {
	p.workingState = p.state.copy()

	p.commit()
	if p.end {
		return
	}
	p.execute()
	p.issue()
	p.renameAndDispatch()
	p.fetchAndDecode()
}

func (p *Processor) latch() {
	p.state = p.workingState
}

func (p *Processor) Simulate(instructions []string, output io.Writer) error {
	err := p.parseInstructions(instructions)
	if err != nil {
		return err
	}

	p.dumpStateIntoLog()

	for !p.end && (p.state.Exception || p.state.PC < uint64(len(p.instructions)) || len(p.state.ActiveList) != 0) {
		p.propagate()

		p.latch()

		p.dumpStateIntoLog()
	}

	return json.NewEncoder(output).Encode(p.log)
}

func New() *Processor {
	var proc Processor

	for i := 0; i < 32; i++ {
		proc.state.RegisterMapTable[i] = PhysReg(i)
	}

	proc.state.FreeList = make([]PhysReg, 0, 32)
	for i := PhysReg(32); i < PhysReg(64); i++ {
		proc.state.FreeList = append(proc.state.FreeList, i)
	}

	return &proc
}
