package lnrpc

import (
	"github.com/pkt-cash/pktd/btcutil/er"
)

// ExtractMinConfs extracts the minimum number of confirmations that each
// output used to fund a transaction should satisfy.
func ExtractMinConfs(minConfs int32, spendUnconfirmed bool) (int32, er.R) {
	switch {
	// Ensure that the MinConfs parameter is non-negative.
	case minConfs < 0:
		return 0, er.New("minimum number of confirmations must " +
			"be a non-negative number")

	// The transaction should not be funded with unconfirmed outputs
	// unless explicitly specified by SpendUnconfirmed. We do this to
	// provide sane defaults to the OpenChannel RPC, as otherwise, if the
	// MinConfs field isn't explicitly set by the caller, we'll use
	// unconfirmed outputs without the caller being aware.
	case minConfs == 0 && !spendUnconfirmed:
		return 1, nil

	// In the event that the caller set MinConfs > 0 and SpendUnconfirmed to
	// true, we'll return an error to indicate the conflict.
	case minConfs > 0 && spendUnconfirmed:
		return 0, er.New("SpendUnconfirmed set to true with " +
			"MinConfs > 0")

	// The funding transaction of the new channel to be created can be
	// funded with unconfirmed outputs.
	case spendUnconfirmed:
		return 0, nil

	// If none of the above cases matched, we'll return the value set
	// explicitly by the caller.
	default:
		return minConfs, nil
	}
}
