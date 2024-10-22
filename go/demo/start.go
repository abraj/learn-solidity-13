package demo

import (
	"libp2pdemo/utils"
	"os"
)

func Start() {
	if len(os.Args) == 1 {
		err := &utils.CustomError{
			Message: "No args provided!",
			Code:    500,
		}
		panic(err)
	}

	nodeNum := os.Args[1]

	if nodeNum == "1" {
		Node1()
	} else if nodeNum == "2" {
		Node2()
	} else if nodeNum == "3" {
		Node3()
	} else {
		err := &utils.CustomError{
			Message: "Invalid arg provided! Expected: 1/2/3",
			Code:    500,
		}
		panic(err)
	}
}
