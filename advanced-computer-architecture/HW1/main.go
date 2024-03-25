package main

import (
	"HW1/processor"
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
	if len(os.Args) != 3 {
		log.Fatalln("./OoO470 </path/to/input.json> </path/to/output.json>")
	}

	outFile, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatalln(err)
	}

	instructions, err := getInstructions(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}

	proc := processor.New()

	if err = proc.Simulate(instructions, outFile); err != nil {
		log.Fatalln(err)
	}
}
