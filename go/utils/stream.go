package utils

import (
	"bufio"
	"strings"

	"github.com/libp2p/go-libp2p/core/network"
)

func ReadStream(s network.Stream) (string, error) {
	// // Approach: 1
	// buf := make([]byte, 256)
	// n, err := s.Read(buf)
	// if err != nil {
	// 	return "", err
	// }
	// data := string(buf[:n])

	// Approach: 2
	buf := bufio.NewReader(s)
	dataRecv, err := buf.ReadString('\n')
	if err != nil {
		return "", err
	}
	data := strings.TrimSuffix(dataRecv, "\n")

	return data, nil
}

func WriteStream(s network.Stream, data string) error {
	// dataSent := data
	dataSent := data + "\n"

	// NOTE: memory-efficient for large JSON data
	// err:= json.NewEncoder(s).Encode(data)

	_, err := s.Write([]byte(dataSent))
	return err
}
