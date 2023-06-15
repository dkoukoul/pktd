# List of RPC commands

The base URL is `http://localhost:8080/api/v1`.

To access help, use `http://localhost:8080/api/v1/help` or for each command separately, e.g., `http://localhost:8080/api/v1/help/lightning/channel`.

## lightning
### Channels

1. List channels - `/lightning/channel`
<details>
<summary>Retrieves a list of currently open channels in the Lightning Network.</summary>

</details>
2. Open channel - `/lightning/channel/open`
   - Initiates the process of opening a new payment channel, allowing the user to establish a direct connection with another participant on the Lightning Network.

3. Close channel - `/lightning/channel/close`
   - Starts the closing procedure for a specific payment channel, allowing the user to request the closure of the channel and settle any outstanding balances.

4. Abandon channel - `/lightning/channel/abandon`
   - Abandons an open but not yet confirmed channel opening transaction, removing it from the mempool and freeing any locked funds.

5. Balance for channel - `/lightning/channel/balance`
   - Retrieves the balance information for a specific payment channel, including the current available balance for each participant.

6. Pending channels - `/lightning/channel/pending`
   - Retrieves information about channels that are in the process of being opened or closed but have not yet reached the confirmed state.

7. Closed channels - `/lightning/channel/closed`
   - Retrieves a list of previously closed payment channels, including details such as the closing transaction ID and the final channel balances.

8. Network info - `/lightning/channel/networkinfo`
   - Retrieves general information about the Lightning Network, including the number of nodes, channels, and the network's current capacity.

9. Fee report - `/lightning/channel/feereport`
   - Provides a report on the fee usage and fee policies for the user's channels, including details on fee rates and fee types.

10. Update channel policy - `/lightning/channel/policy`
    - Allows the user to update the channel policy, which includes parameters such as fee rates, channel reserve, or other channel-specific settings.

11. Export channel backup - `/lightning/channel/backup/export`
    - Initiates the process of exporting a backup of the user's channel state, allowing them to store a copy of the channel's data for recovery purposes.

12. Verify channel backup - `/lightning/channel/backup/verify`
    - Verifies the integrity and validity of a previously exported channel backup, ensuring that it can be safely restored.

13. Restore channel backup - `/lightning/channel/backup/restore`
    - Initiates the process of restoring a previously exported channel backup, allowing the user to recover the channel's state from the backup file.

### Graph

1. Describe graph - `/lightning/graph`
   - Retrieves information about the Lightning Network graph, including details about the network's nodes and channels.

2. Get node metrics - `/lightning/graph/nodemetrics`
   - Retrieves metrics and statistics about the Lightning Network nodes, such as the number of channels, total capacity, and other relevant data.

3. Get channel info - `/lightning/graph/channel`
   - Retrieves detailed information about a specific payment channel, including its ID, participants, capacity, and other channel-specific details.

4. Get node info - `/lightning/graph/nodeinfo`
   - Retrieves information about a specific Lightning Network node, including its node ID, alias, addresses, and other node-specific details.

### Invoice

1. Add invoice - `/lightning/invoice/create`
   - Generates a new invoice, providing the user with a payment request that can be used to receive payments on the Lightning Network.

2. Look up invoice - `/lightning/invoice/lookup`
   - Looks up an invoice by its payment hash, providing information about the invoice's status, amount, and other related details.

3. List invoices - `/lightning/invoice`
   - Retrieves a list of all invoices, including details such as their payment hashes, amounts, and statuses.

4. Decode payment request - `/lightning/invoice/decodepayreq`
   - Decodes a Lightning Network payment request, providing detailed information about the payment amount, description, and other relevant data.

5. Send payment - `/lightning/payment/send`
   - Sends a payment to a Lightning Network node using a payment request, initiating the process of routing the payment through the network.

6. Pay an invoice - `/lightning/payment/payinvoice` (marked TODO streaming)
   - Pays an invoice by its payment hash, completing the payment process and updating the corresponding payment status.

7. Send to route - `/lightning/payment/sendtoroute`
   - Sends a payment along a pre-determined route on the Lightning Network, allowing for more control over the payment path.

8. List payments - `/lightning/payment`
   - Retrieves a list of recent payments made on the Lightning Network, including details such as payment amounts, statuses, and timestamps.

9. Track payment - `/lightning/payment/track` (streaming only) [TODO]
   - Tracks the progress and updates of a specific payment in real-time, providing detailed information about its status and routing.

10. Query routes - `/lightning/payment/queryroutes`
    - Queries the Lightning Network for available routes to a specific destination, providing information about possible paths for routing payments.

11. Forwarding history - `/lightning/payment/fwdinghistory`
    - Retrieves the forwarding history of a Lightning Network node, showing details about incoming and outgoing payments and their corresponding channels.

12. Query mc - `/lightning/payment/querymc`
    - Queries the multi-path payment capabilities of a Lightning Network node, providing information about its supported multi-path routing functionality.

13. Query probability - `/lightning/payment/queryprob`
    - Queries the probability of successful payment routes for a given payment amount, helping to assess the likelihood of successful payment routing.

14. Reset mc - `/lightning/payment/resetmc`
    - Resets the multi-path payment configuration of a Lightning Network node, clearing any previously set payment parameters.

15. Build route - `/lightning/payment/buildroute`
    - Builds a payment route from a source to a destination node, considering various routing parameters and constraints.

### Peer

1. Connect peer - `/lightning/peer/connect`
   - Establishes a connection to a remote Lightning Network peer by specifying its network address, allowing for peer-to-peer communication.

2. Disconnect peer - `/lightning/peer/disconnect`
   - Disconnects from a previously established connection to a Lightning Network peer, terminating the peer-to-peer communication.

3. List peers - `/lightning/peer`
   - Retrieves a list of connected Lightning Network peers, providing information such as their node IDs, network addresses, and connection statuses.

### Meta

1. Debug level - `/meta/debuglevel`
   - Sets the debug log level for the underlying protocol daemon, enabling or disabling specific debug messages for troubleshooting and analysis.

2. MetaService get info - `/meta/getinfo`
   - Retrieves general information about the Lightning Protocol daemon, including its version, network information, and other relevant details.

3. Stop the pld daemon - `/meta/stop`
   - Sends a request to stop the Lightning Protocol daemon gracefully, allowing for proper shutdown and termination of the daemon process.

4. Version - `/meta/version`
   - Retrieves the version information of the Lightning Protocol daemon, providing details about the specific release and version number.

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

2. Change wallet password - `/wallet/changepassphrase`
   - Changes the password used to encrypt and protect the wallet's private keys and sensitive information.

3. Check wallet password - `/wallet/checkpassphrase`
   - Checks the validity of the wallet's password, verifying if it matches the currently set password.

4. Wallet create - `/wallet/create`
   - Creates a new wallet, generating a fresh set of private keys and addresses for receiving funds.

5. Get secret - `/wallet/getsecret`
   - Retrieves the secret key associated with the wallet, allowing for the wallet's recovery or backup.

6. Get Wallet Seed - `/wallet/seed`
   - Retrieves the seed phrase or mnemonic associated with the wallet, allowing for wallet recovery or restoration.

7. Wallet unlock - `/wallet/unlock`
   - Unlocks the wallet by providing the wallet's password, allowing access to funds and performing operations.

8. Get network steward vote - `/wallet/networkstewardvote`
   - Retrieves the network steward vote from the wallet, indicating the current preference or choice for network stewardship.

9. Set network steward vote - `/wallet/networkstewardvote/set`
   - Sets the network steward vote in the wallet, allowing for a change in the preference or choice for network stewardship.

10. Get Transaction - `/wallet/transaction`
    - Retrieves information about a specific transaction in the wallet, including its details, status, and associated addresses.

11. Create transaction - `/wallet/transaction/create`
    - Creates a new transaction, specifying the payment amount, recipient address, and optional parameters for customization.

12. Wallet transactions - `/wallet/transaction/query`
    - Retrieves a list of recent transactions in the wallet, providing details such as transaction IDs, amounts, and timestamps.

13. Send from - `/wallet/transaction/sendfrom`
    - Sends funds from the wallet by specifying the source address and the recipient's address, initiating a payment.

14. Send many - `/wallet/transaction/sendmany`
    - Sends funds to multiple recipients in a single transaction, specifying the amounts and corresponding recipient addresses.

15. Decode transaction - `/wallet/transaction/decode`
    - Decodes a transaction, providing detailed information about its inputs, outputs, fees, and other relevant data.

16. Stop resync - `/wallet/unspent/stopresync`
    - Stops the process of resyncing unspent transaction outputs (UTXOs), which updates the wallet's UTXO data.

17. Delete lock - `/wallet/unspent/lock/delete`
    - Deletes a specific locked unspent transaction output (UTXO) from the wallet, unlocking it for general use.

18. Delete all locks - `/wallet/unspent/lock/deleteall`
    - Deletes all locked unspent transaction outputs (UTXOs) in the wallet, unlocking them for general use.

19. Resync - `/wallet/address/resync`
    - Resynchronizes the wallet's addresses, updating their status and associated information.

20. Stop resync - `/wallet/address/stopresync`
    - Stops the process of resyncing wallet addresses, halting the update of their status and associated information.

21. Get address balances - `/wallet/address/balances`
    - Retrieves the balances of multiple addresses in the wallet, providing the total balance and individual balances per address.

22. New wallet address - `/wallet/address/create`
    - Generates a new address in the wallet, allowing for receiving funds to the newly created address.

23. Dump private key - `/wallet/address/dumpprivkey`
    - Retrieves the private key associated with a specific address in the wallet, allowing for advanced operations.

24. Import private key - `/wallet/address/import`
    - Imports a private key into the wallet, associating it with a specific address for managing funds.

25. Sign message - `/wallet/address/signmessage`
    - Signs a message using the private key associated with a specific address in the wallet, providing cryptographic proof of ownership.

### Neutrino

1. Service bcasttransaction - `/neutrino/bcasttransaction`
   - Broadcasts a signed transaction to the Bitcoin network using the Neutrino protocol.

2. Service estimatefee - `/neutrino/estimatefee`
   - Estimates the transaction fee based on the current state of the Bitcoin network, using the Neutrino protocol.


### Utility

1. Change Passphrase service - `/util/seed/changepassphrase`
   - Changes the passphrase used to encrypt and protect the wallet seed or mnemonic.

2. GenSeed service - `/util/seed/create`
   - Generates a new wallet seed or mnemonic for wallet creation or recovery.

### Watchtower 

1. Create WatchTower - `/wtclient/tower/create`
      - Creates a new Watchtower, which is a dedicated server responsible for monitoring and protecting Lightning Network channels.

2. Remove WatchTower - `/wtclient/tower/remove`
      - Removes a previously created Watchtower from the system.

3. List towers - `/wtclient/tower`
      - Retrieves a list of all existing Watchtowers in the system.

4. Get tower info - `/wtclient/tower/getinfo`
      - Retrieves detailed information about a specific Watchtower, including its configuration and status.

5. Get tower stats - `/wtclient/tower/stats`
      - Retrieves statistical information and metrics about a specific Watchtower's performance.

6. Get tower policy - `/wtclient/tower/policy`
      - Retrieves the policy settings of a specific Watchtower, which determine its behavior and decision-making process.


