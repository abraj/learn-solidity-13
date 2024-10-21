package dial

import "os"

func Start() {
	if len(os.Args) > 1 {
		Node2(os.Args[1])
	} else {
		Node1()
	}
}
