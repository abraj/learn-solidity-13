package demo

import peer "github.com/libp2p/go-libp2p/core/peer"

func GetValidatorsList() []peer.ID {
	// TODO: fetch validator set (registry) from blockchain core contract
	validators := []string{
		"QmaT8zFZp8mKg6dAqxp4wNc7P9dn2WK6imPA37yG8zWwpq",
		// "QmXfjanvuRK2sGZDrKa388ZNHak5DNhkT1Pzibf4YLu5FR",
		"QmQNYyFizjhEFG2Sd4irQS2QvVa7kAwh9mNcsEraSBG4KB",
	}

	// create validator nodes' peerIDs using their string IDs
	var validatorsList []peer.ID
	for _, idStr := range validators {
		peerID, err := peer.Decode(idStr)
		if err != nil {
			panic(err)
		}
		validatorsList = append(validatorsList, peerID)
	}

	return validatorsList
}
