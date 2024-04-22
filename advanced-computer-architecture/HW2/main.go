package main

import (
	"HW2/scheduler"
	"encoding/json"
	"io"
	"log"
	"os"
)

func getInstructions(path string) ([]string, error) {
	inFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer inFile.Close()

	data, err := io.ReadAll(inFile)
	if err != nil {
		return nil, err
	}

	var instructions []string
	err = json.Unmarshal(data, &instructions)
	if err != nil {
		return nil, err
	}

	return instructions, nil
}

func main() {
	if len(os.Args) != 4 {
		log.Fatalln(os.Args[0] + " </path/to/input.json> </path/to/loop.json> </path/to/looppip.json>")
	}

	outLoopFile, err := os.Create(os.Args[3])
	if err != nil {
		log.Fatalln(err)
	}

	outLoopPipFile, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatalln(err)
	}

	instructions, err := getInstructions(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}

	sched := scheduler.New()

	if err = sched.Simulate(instructions, outLoopFile, outLoopPipFile); err != nil {
		log.Fatalln(err)
	}
}
