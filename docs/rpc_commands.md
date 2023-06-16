# List of RPC commands

The base URL is `http://localhost:8080/api/v1`.

To access help, use `http://localhost:8080/api/v1/help` or for each command separately, e.g., `http://localhost:8080/api/v1/help/lightning/channel`.

## lightning
### Channels

1. List channels - `/lightning/channel`
<details>
<summary>Retrieves a list of currently open channels in the Lightning Network.</summary>

#### Request:

* active_only (boolean): If set to true, only active channels will be returned.
* inactive_only (boolean): If set to true, only inactive channels will be returned.
* public_only (boolean): If set to true, only public channels will be returned.
* private_only (boolean): If set to true, only private channels will be returned.
* peer ([]byte): Filters the response for channels with a specific target peer's public key. If the peer field is empty, all channels will be returned.

#### Response:

* channels (array): The list of active channels.
   * active (boolean): Indicates whether the channel is active or not.
   * remote_pubkey ([]byte): The identity pubkey of the remote node.
   * channel_point (string): The outpoint (txid:index) of the funding transaction.
   * chan_id (uint64): The unique channel ID for the channel.
   * capacity (int64): The total amount of funds held in this channel.
   * local_balance (int64): This node's current balance in this channel.
   * remote_balance (int64): The counterparty's current balance in this channel.
   * commit_fee (int64): The amount calculated to be paid in fees for the current set of commitment transactions.
   * commit_weight (int64): The weight of the commitment transaction.
   * fee_per_kw (int64): The required number of satoshis per kilo-weight that the requester will pay at all times.
   * unsettled_balance (int64): The unsettled balance in this channel.
   * total_satoshis_sent (int64): The total number of satoshis sent within this channel.
   * total_satoshis_received (int64): The total number of satoshis received within this channel.
   * num_updates (uint64): The total number of updates conducted within this channel.
   * pending_htlcs (array): The list of active, uncleared HTLCs pending within the channel.
   * incoming (boolean): Indicates if the HTLC is incoming or outgoing.
   * amount (int64): The amount of the HTLC.
   * hash_lock ([]byte): The hash lock of the HTLC.
   * expiration_height (uint32): The expiration height of the HTLC.
   * htlc_index (uint64): Index identifying the HTLC on the channel.
   * forwarding_channel (uint64): The forwarding channel if the HTLC is involved in a forwarding operation.
   * forwarding_htlc_index (uint64): Index identifying the HTLC on the forwarding channel.
   * csv_delay (uint32): The CSV delay expressed in relative blocks.
   * private (boolean): Indicates whether this channel is advertised to the network or not.
   * initiator (boolean): Indicates if the local node was the initiator of the channel.
   * chan_status_flags (string): A set of flags showing the current state of the channel.
   * local_chan_reserve_sat (int64): The minimum satoshis this node is required to reserve in its balance.
   * remote_chan_reserve_sat (int64): The minimum satoshis the other node is required to reserve in its
</details>
2. Open channel - `/lightning/channel/open`
<details>
<summary>Initiates the process of opening a new payment channel, allowing the user to establish a direct connection with another participant on the Lightning Network.</summary>
</details>
3. Close channel - `/lightning/channel/close`
<details>
<summary>Starts the closing procedure for a specific payment channel, allowing the user to request the closure of the channel and settle any outstanding balances.</summary>
</details>

4. Abandon channel - `/lightning/channel/abandon`
<details>
<summary>Abandons an open but not yet confirmed channel opening transaction, removing it from the mempool and freeing any locked funds.</summary>
</details>

5. Balance for channel - `/lightning/channel/balance`
<details>
<summary>Retrieves the balance information for a specific payment channel, including the current available balance for each participant.</summary>
</details>

6. Pending channels - `/lightning/channel/pending`
<details>
<summary>Retrieves information about channels that are in the process of being opened or closed but have not yet reached the confirmed state.</summary>
</details>

7. Closed channels - `/lightning/channel/closed`
<details>
<summary>Retrieves a list of previously closed payment channels, including details such as the closing transaction ID and the final channel balances.</summary>
</details>

8. Network info - `/lightning/channel/networkinfo`
<details>
<summary>Retrieves general information about the Lightning Network, including the number of nodes, channels, and the network's current capacity.</summary>
</details>

9. Fee report - `/lightning/channel/feereport`
<details>
<summary>Provides a report on the fee usage and fee policies for the user's channels, including details on fee rates and fee types.</summary>
</details>

10. Update channel policy - `/lightning/channel/policy`
 <details>
<summary>Allows the user to update the channel policy, which includes parameters such as fee rates, channel reserve, or other channel-specific settings.</summary>
</details>

11. Export channel backup - `/lightning/channel/backup/export`
 <details>
<summary>Initiates the process of exporting a backup of the user's channel state, allowing them to store a copy of the channel's data for recovery purposes.</summary>
</details>

12. Verify channel backup - `/lightning/channel/backup/verify`
 <details>
<summary>Verifies the integrity and validity of a previously exported channel backup, ensuring that it can be safely restored.</summary>
</details>

13. Restore channel backup - `/lightning/channel/backup/restore`
 <details>
<summary>Initiates the process of restoring a previously exported channel backup, allowing the user to recover the channel's state from the backup file.</summary>
</details>

### Graph

1. Describe graph - `/lightning/graph`
<details>
<summary>Retrieves information about the Lightning Network graph, including details about the network's nodes and channels.</summary>
</details>

2. Get node metrics - `/lightning/graph/nodemetrics`
<details>
<summary>Retrieves metrics and statistics about the Lightning Network nodes, such as the number of channels, total capacity, and other relevant data.</summary>
</details>

3. Get channel info - `/lightning/graph/channel`
<details>
<summary>Retrieves detailed information about a specific payment channel, including its ID, participants, capacity, and other channel-specific details.</summary>
</details>

4. Get node info - `/lightning/graph/nodeinfo`
<details>
<summary>Retrieves information about a specific Lightning Network node, including its node ID, alias, addresses, and other node-specific details.</summary>
</details>

### Invoice

1. Add invoice - `/lightning/invoice/create`
<details>
<summary>Generates a new invoice, providing the user with a payment request that can be used to receive payments on the Lightning Network.</summary>
</details>

2. Look up invoice - `/lightning/invoice/lookup`
<details>
<summary>Looks up an invoice by its payment hash, providing information about the invoice's status, amount, and other related details.</summary>
</details>

3. List invoices - `/lightning/invoice`
<details>
<summary>Retrieves a list of all invoices, including details such as their payment hashes, amounts, and statuses.</summary>
</details>

4. Decode payment request - `/lightning/invoice/decodepayreq`
<details>
<summary>Decodes a Lightning Network payment request, providing detailed information about the payment amount, description, and other relevant data.</summary>
</details>

5. Send payment - `/lightning/payment/send`
<details>
<summary>Sends a payment to a Lightning Network node using a payment request, initiating the process of routing the payment through the network.</summary>
</details>

6. Pay an invoice - `/lightning/payment/payinvoice` (marked TODO streaming)
<details>
<summary>Pays an invoice by its payment hash, completing the payment process and updating the corresponding payment status.</summary>
</details>

7. Send to route - `/lightning/payment/sendtoroute`
<details>
<summary>Sends a payment along a pre-determined route on the Lightning Network, allowing for more control over the payment path.</summary>
</details>

8. List payments - `/lightning/payment`
<details>
<summary>Retrieves a list of recent payments made on the Lightning Network, including details such as payment amounts, statuses, and timestamps.</summary>
</details>

9. Track payment - `/lightning/payment/track` (streaming only) [TODO]
<details>
<summary>Tracks the progress and updates of a specific payment in real-time, providing detailed information about its status and routing.</summary>
</details>

10. Query routes - `/lightning/payment/queryroutes`
 <details>
<summary>Queries the Lightning Network for available routes to a specific destination, providing information about possible paths for routing payments.</summary>
</details>

11. Forwarding history - `/lightning/payment/fwdinghistory`
 <details>
<summary>Retrieves the forwarding history of a Lightning Network node, showing details about incoming and outgoing payments and their corresponding channels.</summary>
</details>

12. Query mc - `/lightning/payment/querymc`
 <details>
<summary>Queries the multi-path payment capabilities of a Lightning Network node, providing information about its supported multi-path routing functionality.</summary>
</details>

13. Query probability - `/lightning/payment/queryprob`
 <details>
<summary>Queries the probability of successful payment routes for a given payment amount, helping to assess the likelihood of successful payment routing.</summary>
</details>

14. Reset mc - `/lightning/payment/resetmc`
 <details>
<summary>Resets the multi-path payment configuration of a Lightning Network node, clearing any previously set payment parameters.</summary>
</details>

15. Build route - `/lightning/payment/buildroute`
 <details>
<summary>Builds a payment route from a source to a destination node, considering various routing parameters and constraints.</summary>
</details>

### Peer

1. Connect peer - `/lightning/peer/connect`
<details>
<summary>Establishes a connection to a remote Lightning Network peer by specifying its network address, allowing for peer-to-peer communication.</summary>
</details>

2. Disconnect peer - `/lightning/peer/disconnect`
<details>
<summary>Disconnects from a previously established connection to a Lightning Network peer, terminating the peer-to-peer communication.</summary>
</details>

3. List peers - `/lightning/peer`
<details>
<summary>Retrieves a list of connected Lightning Network peers, providing information such as their node IDs, network addresses, and connection statuses.</summary>
</details>

### Meta

1. Debug level - `/meta/debuglevel`
<details>
<summary>Sets the debug log level for the underlying protocol daemon, enabling or disabling specific debug messages for troubleshooting and analysis.</summary>
</details>

2. MetaService get info - `/meta/getinfo`
<details>
<summary>Retrieves general information about the Lightning Protocol daemon, including its version, network information, and other relevant details.</summary>
</details>

3. Stop the pld daemon - `/meta/stop`
<details>
<summary>Sends a request to stop the Lightning Protocol daemon gracefully, allowing for proper shutdown and termination of the daemon process.</summary>
</details>

4. Version - `/meta/version`
<details>
<summary>Retrieves the version information of the Lightning Protocol daemon, providing details about the specific release and version number.</summary>
</details>

### Wallet

1. Wallet balance - `/wallet/balance`
<details>
<summary>This API endpoint computes and displays the current balance of the wallet. The `WalletBalance` function returns the total unspent outputs (confirmed and unconfirmed), all confirmed unspent outputs, and all unconfirmed unspent outputs under the control of the wallet.</summary>

### Request

The request for this endpoint is of type `rpc_pb_Null`.

### Response

The response for this endpoint is of type `rpc_pb_WalletBalanceResponse` and contains the following fields:

- `total_balance`: The balance of the wallet.
- `confirmed_balance`: The confirmed balance of the wallet (with at least 1 confirmation).
- `unconfirmed_balance`: The unconfirmed balance of the wallet (with 0 confirmations).

All balance fields are of type `int64`.
</details>

2. Wallet change passphrase - `/wallet/changepassphrase`
<details>
<summary>This API endpoint is used to change the password of an encrypted wallet at startup. The `ChangePassword` function changes the password of the encrypted wallet, and if successful, automatically unlocks the wallet database.</summary>

### Request

The request for this endpoint is of type `meta_pb_ChangePasswordRequest` and contains the following fields:

- `current_passphrase`: The current valid passphrase used to unlock the daemon. (Type: string)
- `current_password_bin`: Binary form of `current_passphrase`, if specified, it will override `current_passphrase`. When using JSON, this field must be encoded as base64. (Type: []byte)
- `new_passphrase`: The new passphrase that will be needed to unlock the daemon. (Type: string)
- `new_passphrase_bin`: Binary form of `new_passphrase`, if specified, it will override `new_passphrase`. When using JSON, this field must be encoded as base64. (Type: []byte)
- `wallet_name`: The optional wallet name. If specified, it will override the default `wallet.db`. (Type: string)

### Response

The response for this endpoint is of type `rpc_pb_Null`.
</details>


3. Check wallet password - `/wallet/checkpassphrase`
Wallet check passphrase - `/wallet/checkpassphrase`
<details>
<summary>This API endpoint is used to check the validity of the wallet's password. The `checkpassphrase` function verifies whether the password provided in the request is valid for the wallet.</summary>

### Request

The request for this endpoint is of type `meta_pb_CheckPasswordRequest` and contains the following fields:

- `wallet_passphrase`: The current valid passphrase used to unlock the daemon. (Type: string)
- `wallet_password_bin`: Binary form of `current_passphrase`, if specified, it will override `current_passphrase`. When using JSON, this field must be encoded as base64. (Type: []byte)
- `wallet_name`: The optional wallet name. If specified, it will override the default `wallet.db`. (Type: string)

### Response

The response for this endpoint is of type `meta_pb_CheckPasswordResponse` and contains the following field:

- `valid_passphrase`: A boolean value indicating whether the passphrase is valid for the wallet.
</details>

4. Wallet create - `/wallet/create`
<details>
<summary>This API endpoint is used to initialize a wallet when starting lnd for the first time.</summary>
The `/api/v1/wallet/create` API is used when lnd is starting up for the first time to fully initialize the daemon and its internal wallet. At a minimum, a wallet passphrase must be provided. This passphrase is used to encrypt sensitive material on disk. 

In a recovery scenario, the user can also specify their aezeed mnemonic and passphrase. If set, the daemon will use this prior state to initialize its internal wallet.

Alternatively, this API can be used along with the `/util/seed/create` API to obtain a seed. Once the seed has been verified by the user, it can be fed into this API to commit the new wallet.

## Request

The request should be a JSON object with the following fields:

- `wallet_passphrase` (string, required): The passphrase that should be used to encrypt the wallet. It must be at least 8 characters in length. After creation, this password is required to unlock the daemon.

- `wallet_passphrase_bin` ([]byte, optional): If specified, it will override `wallet_passphrase`, but is expressed in binary. When using REST, this field must be encoded as base64.

- `wallet_seed` (string array, optional): A 15-word wallet seed. This may have been generated by the `GenSeed` method or be an existing seed.

- `seed_passphrase` (string, optional): An optional user-provided passphrase that will be used to encrypt the generated seed.

- `seed_passphrase_bin` ([]byte, optional): If specified, it will override `seed_passphrase`, but is expressed in binary. When using REST, this field must be encoded as base64.

- `recovery_window` (int32, optional): An optional argument specifying the address lookahead when restoring a wallet seed. The recovery window applies to each individual branch of the BIP44 derivation paths. Supplying a recovery window of zero indicates that no addresses should be recovered, such as after the first initialization of the wallet.

- `channel_backups` (object, optional): An optional argument that allows clients to recover the settled funds within a set of channels. This should be populated if the user was unable to close out all channels and sweep funds before partial or total data loss occurred. If specified, after on-chain recovery of funds, lnd will begin to carry out the data loss recovery protocol to recover the funds in each channel from a remote force closed transaction.

- `wallet_name` (string, optional): An optional argument that allows defining the wallet filename other than the default `wallet.db`.

## Response

The response is a JSON object with the following field:

- `null`: Indicates a successful wallet initialization.
</details>

5. Get secret - `/wallet/getsecret`
<details>
<summary></summary>
</details>

6. Get Wallet Seed - `/wallet/seed`
<details>
<summary>This provides which is generated using the wallet's private keys, this can be used as a password for another application. It will be the same as long as this wallet exists, even if it is re-recovered from seed.</summary>
</details>

7. Wallet unlock - `/wallet/unlock`
<details>
<summary>Unlocks the wallet by providing the wallet's password, allowing access to funds and performing operations.</summary>
</details>

8. Get network steward vote - `/wallet/networkstewardvote`
<details>
<summary>Retrieves the network steward vote from the wallet, indicating the current preference or choice for network stewardship.</summary>
</details>

9. Set network steward vote - `/wallet/networkstewardvote/set`
<details>
<summary>Configure the wallet to vote for a network steward</summary>

#### Request

The request should be a JSON object with the following fields:

- `vote_against` (string): PKT address to vote against.

- `vote_for` (string): PKT address to vote for.

```json
{
  "vote_against": "pkt1...",
  "vote_for": "pkt1..."
}
```
#### Response
Empty

</details>

10. Get Transaction - `/wallet/transaction`
 <details>
<summary>Retrieves information about a specific transaction in the wallet.</summary>

#### Request
```json
   {
      "txid": "06808e49f550720b6142da9b2b30bd298eefa10ee388d1d8d65655710847fcac",
      "includewatchonly": true
   }
```
#### Response

```json
{
	"transaction":  {
		"amount":  398.66999986581504,
		"amountUnits":  "428068652830",
		"fee":  1.341104507446289e-7,
		"feeUnits":  "144",
		"confirmations":  "294050",
		"blockHash":  "154cf1b5cf389128f0e463160e0159a6be45e72e1c67855ad2e7dacaf987fffb",
		"blockTime":  "1669193977",
		"txid":  "06808e49f550720b6142da9b2b30bd298eefa10ee388d1d8d65655710847fcac",
		"time":  "1669193977",
		"timeReceived":  "1669193977",
		"details":  [
			{
				"amount":  -400,
				"category":  "send",
				"amountUnits":  "429496729600"
			},
			{
				"address":  "pkt1q06xj0j263uec0qmr9n3enwx54sgyxt02wql675",
				"amount":  398.66999986581504,
				"category":  "receive"
			}
		],
		"raw":  "AQAAAAABAT7RAzHrXE2TpJMF4GVijw4QWqBBFZyzkvoKl1O5St97AQAAAAD/////Ah5H4apjAAAAFgAUfo0nyVqPM4eDYyzjmbjUrBBDLepSuB5VAAAAABYAFCVyjLocXftQgN0c2pGV1URnUd71AkgwRQIhAIBh9hCkkIOX5miq/dCaJU5BjZUxY65oeqMHtnOoMYbgAiBS4NfrumkGXQkMGIwxrl99Lbm05G3h1d1cLClR8+6cigEhAgdQFDpEsow07WpMhnidHdSrosOuN8CZEMMSK9TCMQBeAAAAAA=="
	}
}
```
</details>

11. Create transaction - `/wallet/transaction/create`
 <details>
<summary>Create a transaction but do not send it to the chain. This does not store the transaction as existing in the wallet so ```/wallet/transaction/query``` will not return a transaction created by this endpoint. In order to make multiple transactions concurrently, prior to the first transaction being submitted to the chain, you must specify the autolock field.</summary>
</details>

#### Request
* to_address: The address to which the payment will be made. (Type: string)
* amount: The number of PKT (crypto token) to send. Use Infinity to send as much as possible in a single transaction. (Type: float64)
* from_address (repeated): Addresses from which funds can be sourced. (Type: array of strings)
* electrum_format: Output an electrum format transaction. This format carries additional payload for enabling offline transactions, including multi-signature. (Type: boolean)
* change_address: If not an empty string, this address will be used for making change. (Type: string)
* input_min_height: Do not source funds from any transaction outputs with a block height less than this value. (Type: int32)
* min_conf: Do not source funds from any transaction outputs unless they have at least this many confirmations. (Type: int32)
* max_inputs: Do not use more than this number of previous transaction outputs as inputs to source funds. (Type: int32)
* autolock: Create a "named lock" for all outputs to be spent. This allows you to prevent further invocations of creating a transaction from referencing the same coins. The name is your * choice. The locked outputs will be unlocked on wallet restart, by using wallet/unspent/lock/create with unlock = true, or if the transaction is sent to the chain (in which case they become permanently unusable). (Type: string)
* sign: Specify whether to sign the transaction. (Type: boolean)

Example:
```json
{
  "to_address":"pkt1qy4egewsutha4pqxarndfr9w4g3n4rhh47amu5u",
  "amount":10,
  "from_address":["pkt1q06xj0j263uec0qmr9n3enwx54sgyxt02wql675"],
  "sign": true
}
```
#### Response

Example:
```json
{
	"transaction":  "AQAAAAABAaz8RwhxVVbW2NGI4w6h744pvTArm9pCYQtyUPVJjoAGAAAAAAD/////Ao5G4SphAAAAFgAUfo0nyVqPM4eDYyzjmbjUrBBDLeoAAACAAgAAABYAFCVyjLocXftQgN0c2pGV1URnUd71AkgwRQIhAIC4nLdrrBiBFcqpE6pBh6QMJodmDcei0wMs5D9XCeZ2AiBuTVwFhjUmVM1M78Ju8huTIpR5zkHOwWhuQgz/JOYgrQEhAgdQFDpEsow07WpMhnidHdSrosOuN8CZEMMSK9TCMQBeAAAAAA=="
}
```
12. Wallet transactions - `/wallet/transaction/query`
 <details>
<summary>List transactions from the wallet. Returns a list describing all the known transactions relevant to the wallet. Includes confirmed (in the chain) transactions and unconfirmed (mempool) transactions. Excludes transactions made with /wallet/transaction/create but not yet broadcasted to the network. Excludes transactions that are not known to be relevant to the wallet. If transactions are missing, a resync may be necessary.</summary>

#### Request

* start_height (type: int32): The height from which to list transactions, inclusive.
* end_height (type: int32): The height until which to list transactions, inclusive. To include unconfirmed transactions, set this value to -1. If no end_height is provided, the call will default to this option.
* txns_limit (type: int32): Return no more than this number of transactions within the specified height range.
* txns_skip (type: int32): Skip this number of the first transactions to allow windowing over the set of all transactions.
* coinbase (type: rpc_pb_CoinbaseSelector): Whether to include, exclude, or only provide coinbase (mining) transactions.
* reversed (type: bool): If set, the payments returned will result from seeking backward from the specified index offset. Used for pagination in reverse order.
* vin_detail (type: bool): If true, the transactions will include exact details of every input.
* tx_bin (type: bool): If true, the result will include the binary representation of the transactions.

Example:
```json
{
  "coinbase":1,
  "reversed": false,
  "txnsSkip":0,
  "txnsLimit":20,
  "endHeight": -1
}
```
#### Response

Example:
```json
{
	"transactions":  [
		{
			"tx":  {
				"txid":  "996a3c7a6113072b46863c1577b67d0df2f5c92c23faa043e67c9c5831a3b350",
				"version":  1,
				"sfee":  "unknown",
				"size":  222,
				"vsize":  141,
				"payers":  [
					{
						"address":  "pkt1qfy0ap2a23xu4pn4wf3m73w4lyas2uh83gugxkr",
						"inputs":  1,
						"valueCoins":  "NaN",
						"svalue":  "unknown"
					}
				],
				"vout":  [
					{
						"valueCoins":  26404.341245530173,
						"svalue":  "28351445530494",
						"address":  "pkt1q7375luf6ln7d0xvlmp5jh4umdkjwyk8kxdy50a"
					},
					{
						"valueCoins":  24187.658744201995,
						"svalue":  "25971300818289",
						"n":  1,
						"address":  "pkt1qvqpdf4fyygn9rguq56yve6hfv5x9nlj6uazzc6"
					}
				]
			},
			"numConfirmations":  627444,
			"blockHash":  "ce105233a9e6b9d9d4af42151e9248f34000e4dd4a8d94d12a72b01a328b4692",
			"blockHeight":  1391222,
			"time":  "1648974267"
		}
	]
}
```
</details>

13. Send from - `/wallet/transaction/sendfrom`
 <details>
<summary>Authors, signs, and sends a transaction which sources funds from specific addresses</summary>

#### Request

* to_address (string): The address to send funds to.
amount (float64): The amount of coins to send, denominated in whole PKT. Setting the value to "Infinity" means sending as much as possible. The maximum amount possible depends on fees and the limited maximum number of inputs. To completely sweep an address, you may need to call this endpoint multiple times.
* from_address (string array): A list of addresses or UTXO identifiers (i.e., TXID + ":" + output number) to source funds from. You can mix and match addresses and identifiers.
* min_conf (int32): The minimum number of confirmations required for the payment from which funds are sourced. By default, 1 is used, meaning any payment in the blockchain.
* max_inputs (int32): The maximum number of inputs to use for sourcing funds. By default, 0 means no limit.
* min_height (int32): The minimum block height for sourcing funds. Payments older (lower block height) than this number will not be used. The default is 0, indicating no limit.

Example:
```json
{
  "to_address":"pkt1qy4egewsutha4pqxarndfr9w4g3n4rhh47amu5u",
  "amount": 10,
  "min_conf":0,
  "from_address":["pkt1q06xj0j263uec0qmr9n3enwx54sgyxt02wql675"]
}
```

#### Response

Example:
```json
{
	"txHash":  "3466ce6bd2ea28abb36cae9a7185fe3fa3fd43ff3388669746db77e7f7a8b517"
}
```
</details>

14. Send many - `/wallet/transaction/sendmany`
 <details>
<summary>Sends funds to multiple recipients in a single transaction, specifying the amounts and corresponding recipient addresses.</summary>
</details>

15. Decode transaction - `/wallet/transaction/decode`
 <details>
<summary>Decodes a transaction, providing detailed information about its inputs, outputs, fees, and other relevant data.</summary>
</details>

16. Unspent - `/wallet/unspent`
<details><summary>
This endpoint returns a list of UTXOs spendable by the wallet, filtered by the specified minimum and maximum number of confirmations.
</summary>

#### Request
* min_confs (integer): The minimum number of confirmations required for UTXOs to be included in the list.
* max_confs (integer): The maximum number of confirmations allowed for UTXOs to be included in the list.

Example:
```json
{
  "min_confs":0,
  "max_confs": 1000
}
```
#### Response

* address_type: The type of address associated with the UTXO, which can be either "p2wkh" (Pay to Witness Key Hash) or "np2wkh" (Pay to Nested Witness Key Hash).
* address: The address associated with the UTXO.
* amount_sat: The value of the unspent coin in satoshis.
* pk_script: The public key script in hexadecimal format.
* outpoint: The outpoint identifying the UTXO, specified as txid:n, where txid represents the transaction ID and n represents the output index.
* confirmations: The number of confirmations for the UTXO.

Example:
```json
{
	"utxos":  [
		{
			"address":  "pkt1q06xj0j263uec0qmr9n3enwx54sgyxt02wql675",
			"amountSat":  "406593816062",
			"pkScript":  "00147e8d27c95a8f338783632ce399b8d4ac10432dea",
			"outpoint":  {
				"txidBytes":  "F7Wo9+d320aXZogz/0P9oz/+hXGarmyzqyjq0mvOZjQ=",
				"txidStr":  "3466ce6bd2ea28abb36cae9a7185fe3fa3fd43ff3388669746db77e7f7a8b517"
			},
			"confirmations":  "24"
		}
	]
}
```
</details>

17. Unspent lock - `/wallet/unspent/lock`
<details>
<summary>This endpoint returns a set of outpoints (UTXOs) that have been marked as locked using the /wallet/unspent/lock/create endpoint. The locked UTXOs are grouped by a lock name.</summary>

#### Request
Empty
#### Response
* lock_name (string): The name of the lock group. An empty string represents uncategorized locks.
* utxos (repeated rpc_pb_OutPoint): The UTXOs belonging to this lock group. Each UTXO is represented by rpc_pb_OutPoint, which has the following fields:
* txid_bytes ([]byte): Raw bytes representing the transaction ID.
* txid_str (string): Reversed, hex-encoded string representing the transaction ID.
* output_index (uint32): The index of the output on the transaction.

</details>

18. Unspent lock create - `/wallet/unspent/lock/create`
<details>
<summary>This endpoint allows you to lock one or more UTXOs. You can optionally specify a group name, and you can call this endpoint multiple times with the same group name to add more UTXOs to the group. It's important to note that the lock group name "none" is reserved.</summary>

#### Request

* transactions (repeated rpc_pb_OutPoint): The UTXOs to lock. Each UTXO is represented by rpc_pb_OutPoint, which has the following fields:
* txid_bytes ([]byte): Raw bytes representing the transaction ID.
* txid_str (string): Reversed, hex-encoded string representing the transaction ID.
* output_index (uint32): The index of the output on the transaction.
* lockname (string): An optional lock name to assign to the locked UTXOs. This allows them to be batch-unlocked later. If the lockname is an empty string, it will be disregarded.
#### Response

</details>

19. Delete lock - `/wallet/unspent/lock/delete`
<details>
<summary>Deletes a specific locked unspent transaction output (UTXO) from the wallet, unlocking it for general use.</summary>
</details>

20. Delete all locks - `/wallet/unspent/lock/deleteall`
 <details>
<summary>Deletes all locked unspent transaction outputs (UTXOs) in the wallet, unlocking them for general use.</summary>
</details>

21. Resync - `/wallet/address/resync`
 <details>
<summary>Resynchronizes the wallet's addresses, updating their status and associated information.</summary>
</details>

22. Stop resync - `/wallet/address/stopresync`
 <details>
<summary>Stops the process of resyncing wallet addresses, halting the update of their status and associated information.</summary>
</details>

23. Get address balances - `/wallet/address/balances`
 <details>
<summary>Retrieves the balances of multiple addresses in the wallet, providing the total balance and individual balances per address.</summary>
</details>

#### Request

Example:
```json
{
  "showzerobalance":false
}
```

#### Response

Example:
```json
{
	"addrs":  [
		{
			"address":  "pkt1q06xj0j263uec0qmr9n3enwx54sgyxt02wql675",
			"total":  378.66999959759414,
			"stotal":  "406593816062",
			"spendable":  378.66999959759414,
			"sspendable":  "406593816062",
			"outputcount":  1
		},
		{
			"address":  "pkt1q7375luf6ln7d0xvlmp5jh4umdkjwyk8kxdy50a",
			"total":  28304.341245530173,
			"stotal":  "30391554996094",
			"spendable":  28304.341245530173,
			"sspendable":  "30391554996094",
			"outputcount":  3
		}
	]
}
```
24. New wallet address - `/wallet/address/create`
<details>
<summary>This endpoint is used to generate a new payment address.</summary>

#### Request
* legacy (bool): A boolean flag indicating whether to generate a legacy address. The legacy addresses are the older address format used in Bitcoin. If set to true, a legacy address will be generated.

#### Response
Example:
```json
{
	"address":  "pkt1qzyvyp5vmk73cgz9g78fge6efkd5glhamcmxffq"
}
```

</details>

25. Dump private key - `/wallet/address/dumpprivkey`
 <details>
<summary>This endpoint returns the private key in Wallet Import Format (WIF) encoding that controls a specific wallet address. It's important to note that if the private key falls into the wrong hands, all funds associated with that address can be stolen. However, other addresses in the wallet are not affected.</summary>
</details>

#### Request

* address (string): The wallet address for which the private key is to be retrieved.

#### Response

* private_key (string): The private key associated with the specified address in Wallet Import Format (WIF) encoding.

24. Import private key - `/wallet/address/import`
 <details>
<summary>This endpoint allows you to import a private key (WIF-encoded) into the wallet. Once imported, funds associated with this key/address become spendable. It's important to note that imported addresses will not be recovered if you recover your wallet from a seed since they are not mathematically derived from the seed.</summary>

#### Request
* private_key (string): The private key in Wallet Import Format (WIF) encoding to be imported.
* rescan (bool): A flag indicating whether to rescan the blockchain for transactions involving the imported key/address (optional).
* legacy (bool): A flag indicating whether the private key corresponds to a legacy address (optional).

#### Response

* address (string): The wallet address associated with the imported private key.

</details>

25. Sign message - `/wallet/address/signmessage`
<details>
<summary>This endpoint allows you to sign a message using the private key of a payment address. The resulting signature string can be verified using a utility such as "pkt-checksig" (https://github.com/cjdelisle/pkt-checksig). It's important to note that only legacy style addresses (mixed capital and lowercase letters, beginning with a 'p') can currently be used to sign messages.</summary>

#### Request
* msg (string): The message to be signed.
* msg_bin ([]byte): If specified, it will override the msg field and provide the binary form of the message (optional).
* address (string): The address to select for signing with.
#### Response

* signature (string): The signature for the given message.

</details>

### Neutrino

1. Service bcasttransaction - `/neutrino/bcasttransaction`
<details>
<summary> This endpoint allows you to broadcast a transaction to the network so that it can be logged in the chain.</summary>

#### Request

* tx ([]byte): The transaction to be broadcasted, represented as a byte array.

#### Response

* txn_hash (string): The hash of the transaction that was successfully broadcasted.

</details>

2. Service estimatefee - `/neutrino/estimatefee`
<details>
<summary>Estimates the transaction fee based on the current state of the Bitcoin network, using the Neutrino protocol.</summary>
</details>


### Utility

1. Change Passphrase service - `/util/seed/changepassphrase`
<details>
<summary>Changes the passphrase used to encrypt and protect the wallet seed or mnemonic.</summary>
</details>

2. GenSeed service - `/util/seed/create`
<details>
<summary>Generates a new wallet seed or mnemonic for wallet creation or recovery.</summary>
</details>

### Watchtower 

1. Create WatchTower - `/wtclient/tower/create`
   <details>
<summary>Creates a new Watchtower, which is a dedicated server responsible for monitoring and protecting Lightning Network channels.</summary>
</details>

2. Remove WatchTower - `/wtclient/tower/remove`
   <details>
<summary>Removes a previously created Watchtower from the system.</summary>
</details>

3. List towers - `/wtclient/tower`
   <details>
<summary>Retrieves a list of all existing Watchtowers in the system.</summary>
</details>

4. Get tower info - `/wtclient/tower/getinfo`
   <details>
<summary>Retrieves detailed information about a specific Watchtower, including its configuration and status.</summary>
</details>

5. Get tower stats - `/wtclient/tower/stats`
   <details>
<summary>Retrieves statistical information and metrics about a specific Watchtower's performance.</summary>
</details>

6. Get tower policy - `/wtclient/tower/policy`
   <details>
<summary>Retrieves the policy settings of a specific Watchtower, which determine its behavior and decision-making process.</summary>
</details>
