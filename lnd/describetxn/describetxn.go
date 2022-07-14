package describetxn

import (
	"math"
	"strconv"

	"github.com/pkt-cash/pktd/blockchain"
	"github.com/pkt-cash/pktd/btcutil"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/chaincfg"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/mempool"
	"github.com/pkt-cash/pktd/pktlog/log"
	"github.com/pkt-cash/pktd/txscript"
	"github.com/pkt-cash/pktd/wire"
)

// Describe creates a TransactionInfo from a transaction
// @param getTxns a closure which will populate a map of transaction by hex txid. This is best-effort,
//                if it does not manage to supply some of the transactions there will be "unknown" input
//                amounts and fees, see lnrpc.VinDetail.ValueCoins for more information.
// @param mtx the transaction
// @param chainParams the chain which we are using, for things like address format
// @param includeVinDetail if true then the result will include detailed info on each input lnrpc.VinDetail
// @return the transaction info or an error
func Describe(
	getTxns func(map[string]*wire.MsgTx) er.R,
	mtx wire.MsgTx,
	chainParams *chaincfg.Params,
	includeVinDetail bool,
) (*rpc_pb.TransactionInfo, er.R) {

	vin, err := createVinListPrevOut(getTxns, &mtx, chainParams)
	if err != nil {
		return nil, err
	}

	sfee := "unknown"
	if true {
		failed := false
		fee := int64(0)
		for _, input := range vin {
			if math.IsNaN(input.ValueCoins) {
				failed = true
				break
			}
			n, errr := strconv.ParseInt(input.Svalue, 10, 64)
			if errr != nil {
				return nil, er.E(errr)
			}
			fee += n
		}
		for _, out := range mtx.TxOut {
			fee -= out.Value
		}
		if !failed {
			sfee = strconv.FormatInt(fee, 10)
		}
	}

	payers := getPayers(vin)
	if !includeVinDetail {
		vin = nil
	}

	return &rpc_pb.TransactionInfo{
		Txid:      mtx.TxHash().String(),
		Version:   mtx.Version,
		Locktime:  mtx.LockTime,
		Size:      int32(mtx.SerializeSize()),
		Vsize:     int32(mempool.GetTxVirtualSize(btcutil.NewTx(&mtx))),
		Payers:    payers,
		VinDetail: vin,
		Vout:      createVoutList(&mtx, chainParams),
		Sfee:      sfee,
	}, nil
}

// createVinListPrevOut returns a slice of JSON objects for the inputs of the
// passed transaction.
func createVinListPrevOut(
	getTxns func(map[string]*wire.MsgTx) er.R,
	mtx *wire.MsgTx,
	chainParams *chaincfg.Params,
) ([]*rpc_pb.VinDetail, er.R) {
	vinFullList := createVinList(mtx, chainParams)
	if err := loadPrevOuts(getTxns, mtx, chainParams, vinFullList); err != nil {
		return nil, err
	}
	return vinFullList, nil
}

func loadPrevOuts(
	getTxns func(map[string]*wire.MsgTx) er.R,
	mtx *wire.MsgTx,
	chainParams *chaincfg.Params,
	list []*rpc_pb.VinDetail,
) er.R {
	if blockchain.IsCoinBaseTx(mtx) {
		// By definition, a coinbase tx has only one input which cannot be loaded
		return nil
	}

	hashes := make(map[string]*wire.MsgTx)
	for _, vin := range list {
		hashes[vin.Txid] = nil
	}
	if err := getTxns(hashes); err != nil {
		return err
	}

	for _, vin := range list {
		txn := hashes[vin.Txid]
		if txn == nil {
			// We were not able to get this one from the db
			continue
		}
		relevantOutput := txn.TxOut[vin.Vout]
		a := txscript.PkScriptToAddress(relevantOutput.PkScript, chainParams).EncodeAddress()
		if vin.Address != "unknown" && vin.Address != a {
			log.Warn("For txin %s:%d - computed address is %s but prev txn address is %s",
				vin.Txid, vin.Vout, vin.Address, a)
		}
		vin.Address = a
		vin.ValueCoins = btcutil.Amount(relevantOutput.Value).ToBTC()
		vin.Svalue = strconv.FormatInt(relevantOutput.Value, 10)
	}

	return nil
}

// createVinList returns a slice of JSON objects for the inputs of the passed
// transaction.
func createVinList(mtx *wire.MsgTx, chainParams *chaincfg.Params) []*rpc_pb.VinDetail {
	// Coinbase transactions only have a single txin by definition.
	if blockchain.IsCoinBaseTx(mtx) {
		txIn := mtx.TxIn[0]
		return []*rpc_pb.VinDetail{
			{
				Coinbase: txIn.SignatureScript,
				Sequence: txIn.Sequence,
			},
		}
	}

	vinList := make([]*rpc_pb.VinDetail, 0, len(mtx.TxIn))

	for i, txIn := range mtx.TxIn {
		// Create the basic input entry without the additional optional
		// previous output details which will be added later if
		// requested and available.
		prevOut := &txIn.PreviousOutPoint
		vinEntry := rpc_pb.VinDetail{
			Txid:       prevOut.Hash.String(),
			Vout:       prevOut.Index,
			Sequence:   txIn.Sequence,
			ValueCoins: math.NaN(),
			Svalue:     "unknown",
			Address:    "unknown",
		}
		if len(txIn.SignatureScript) > 0 {
			// The disassembled string will contain [error] inline
			// if the script doesn't fully parse, so ignore the
			// error here.
			vinEntry.ScriptSig = txIn.SignatureScript

			// If we have a sigscript then we may be able to deduce the
			// address which was paid by popping the key
			addr := txscript.SigScriptToAddress(txIn.SignatureScript, chainParams)
			if addr != nil {
				vinEntry.Address = addr.EncodeAddress()
			}
		}

		// Electrum formatted transactions contain this additional information
		// i.e. value and PkScript
		if len(mtx.Additional) == len(mtx.TxIn) {
			if mtx.Additional[i].Value != nil {
				v := *mtx.Additional[i].Value
				vinEntry.ValueCoins = btcutil.Amount(v).ToBTC()
				vinEntry.Svalue = strconv.FormatInt(v, 10)
			}
			if len(mtx.Additional[i].PkScript) > 0 {
				s := mtx.Additional[i].PkScript
				a := txscript.PkScriptToAddress(s, chainParams).EncodeAddress()
				vinEntry.Address = a
			}
		}

		if len(txIn.Witness) != 0 {
			vinEntry.Witness = txIn.Witness
			// The witness provides a means to deduce the address (currently) in every case
			addr := txscript.WitnessToAddress(txIn.Witness, chainParams)
			if addr != nil {
				vinEntry.Address = addr.EncodeAddress()
			}
		}

		vinList = append(vinList, &vinEntry)
	}

	return vinList
}

func vote(voteOut **rpc_pb.Vote, script []byte, params *chaincfg.Params) {
	voteFor, voteAgainst := txscript.ElectionGetVotesForAgainst(script)
	if voteFor == nil && voteAgainst == nil {
		return
	}
	v := rpc_pb.Vote{}
	if voteFor != nil {
		v.For = txscript.PkScriptToAddress(voteFor, params).EncodeAddress()
	}
	if voteAgainst != nil {
		v.Against = txscript.PkScriptToAddress(voteAgainst, params).EncodeAddress()
	}
	*voteOut = &v
}

// createVoutList returns a slice of JSON objects for the outputs of the passed
// transaction.
func createVoutList(mtx *wire.MsgTx, chainParams *chaincfg.Params) []*rpc_pb.Vout {
	voutList := make([]*rpc_pb.Vout, 0, len(mtx.TxOut))
	for i, v := range mtx.TxOut {
		encodedAddr := txscript.PkScriptToAddress(v.PkScript, chainParams).EncodeAddress()

		vout := &rpc_pb.Vout{
			N:          uint32(i),
			ValueCoins: btcutil.Amount(v.Value).ToBTC(),
			Svalue:     strconv.FormatInt(v.Value, 10),
			Address:    encodedAddr,
		}

		vote(&vout.Vote, v.PkScript, chainParams)
		voutList = append(voutList, vout)
	}

	return voutList
}

// Simplify the VinDetail list into a list of Payers
// You should call loadPrevOuts first, otherwise you will definitely
// not have any ValueCoins or Svalue.
func getPayers(list []*rpc_pb.VinDetail) []*rpc_pb.Payer {
	payerByAddress := make(map[string]*rpc_pb.Payer)
	for _, vd := range list {
		payer, ok := payerByAddress[vd.Address]
		if !ok {
			p := rpc_pb.Payer{
				Address:    vd.Address,
				Inputs:     0,
				ValueCoins: 0.0,
				Svalue:     "0",
			}
			payerByAddress[vd.Address] = &p
			payer = &p
		}
		payer.Inputs += 1
		if payer.Svalue != "unknown" {
			if vd.Svalue == "unknown" {
				// We are nolonger sure of the value
				payer.Svalue = "unknown"
				payer.ValueCoins = math.NaN()
			} else if psvalue, err := strconv.ParseInt(payer.Svalue, 10, 64); err != nil {
				// should never happen
				panic("unable to parse payer.Svalue")
			} else if vdsvalue, err := strconv.ParseInt(vd.Svalue, 10, 64); err != nil {
				// should never happen
				panic("unable to parse vd.Svalue")
			} else {
				// add the amount sourced from this input
				psvalue += vdsvalue
				payer.ValueCoins = btcutil.Amount(psvalue).ToBTC()
				payer.Svalue = strconv.FormatInt(psvalue, 10)
			}
		}
	}
	payers := make([]*rpc_pb.Payer, 0, len(payerByAddress))
	for _, p := range payerByAddress {
		payers = append(payers, p)
	}
	return payers
}
