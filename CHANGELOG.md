# Release version pktd-v1.7.0
TODO Not yet released

## Major changes

### New RPC util/transaction/decode
This RPC provides a way to decode hex or binary format of a transaction and print
everything that the wallet knows about it, including source addresses and amounts/fees
if possible.

### Compute the sender address for incoming transactions
As surprising as it may sound, it can be very difficult to figure out who sent you money!
The way the PKT, and BTC blockchains work is that each transaction sources funds from
certain "inputs" (references to previous transactions) and then sends those funds to certain
"outputs" (public signing keys). The "inputs" contain cryptographic signatures in order to
prove validity of the spending of the funds, but they do not make it easy to determine the
address from which coins are being spent, and the amount of coins is missing entirely.

The block explorer and pktd are able to easily work around this problem because they have
the entire blockchain on disk so they can simply access the previous transaction and read
the output from it. But for the wallet, accessing the previous transaction is impossible
without changing the p2p protocol or adding a web service.

In this release, we added code to compute the address from the signature, so we now are
able to see what address(es) contributed to a transaction which paid you. We still can't
always know the *amount* of a transaction input (we only know it if we are the PAYER, thus
we have the previous transaction in our db already), but we can reliably know the address.

Old payments will still be missing the input address but this can be fixed with a resync.

### Better peer selection
PLD and pktwallet both have had significant peer selection issues. They would eventually
connect but it would take time before they did so. This was the fault of a number of
different bugs. Normally the peer connection logic should cycle through IP addresses which
may contain pktd nodes and attempt to connect to them, keeping track of which ones work and
which ones don't, and preferring nodes which have worked in the past, and fresh IP addresses
which were never tried before.

1. Addresses which were tested but were down or unreachable were not being accounted as
having been tested.
2. When trying to connect to a node, the wallet would pause for 120 seconds waiting for a
reply, seriously slowing down the process of finding good nodes.
3. The number of nodes to connect to was set to 8, which is also the maximum number of
allowable connections, causing the wallet code to churn connection and disconnection cycles.
4. The wallet treated all nodes as equal, whether IPv4, IPv6, cjdns, or Yggdrasil based,
now it tests to see what types of nodes it can actually connect to.

The newly reworked code is able to find 5 peers within 4 seconds of FIRST startup, with no
known peers. In a subsequent restart, it's connected within 2 seconds.

### Improve performance of UTXO scan
Scanning over Unspent Transaction Outputs is necessary in order to make a payment or
compute the balance of an address. Prior to this release, the scan took a fair amount of time
because the data was stored in multiple places - so multiple disk accesses were required for
each unspent transaction output. This improvement changes how the data is represented in the
wallet.db file, improving the speed of getaddressbalances by 10x, and the speed of sending
a transaction by as much as 40x.

## Minor changes
1. Fix startup log to indicate how to properly unlock the wallet
59c46476f99b08662fcf8293fef5756eeb2ae913
2. Pause queryAllPeers until there are peers (reduce "query failed" errors)
81bfdfa8d207211f0997b315c50b5b78f2402e6c
3. Don't try to query when we have no sync peer, so we don't get failed queries (reduce
"query failed" errors) 36ff908d3df09ebe294cfadb119cb4ff2e41c1ad
4. Improve performance and logging for rescans
36225b2a8fafaa8da53f7acbf876471b6a4cd906

## Stats
TODO

## Future Goals
TODO

## Thanks!
TODO

# Release version pktd-v1.6.0
July 5, 2022

## Major changes

### New RPC system in pld
New RPC system. The RPC system of LND is GRPC, the problem with GRPC is that it's
difficult to use and the software is not maintained as well as the developers at Google
would have you believe.

In one example, the Java GRPC client can trigger a crash (nil deref segfault) in the Golang
GRPC server by making a request. It's worth noting that a mature RPC library should be
impossible to crash, even intentionally with malicious requests. The fact that the Golang
GRPC server is crashable not just by a malformed request but in fact by a normal request
made by another GRPC library tells us that there's a total lack of quality assurance.

Secondly, the GRPC client has abysmal performance on Android, and the iOS AnodeVPN team
was not able to get it working in their React Native stack, at all.

But GRPC is not just buggy and difficult to integrate, the API is also "opinionated" in
that it requires the use of it's own internal HTTP/2 server rather than offering an API
by which request data can be passed in. This makes it impossible to integrate into an
existing server which also does other things, and makes it married to the HTTP/2 protocol
which is not always available, for example in a web browser based client without HTTPS
encryption, or in our case, in a React Native based app.

Finally, GRPC is not at all conducive to testing and debugging. It relies exclusively on
Protocol Buffers binary representation of data, which is nice for sending data around in
production, but you can't just make requests using a tool like curl.

So we replaced GRPC with a simple http REST server, it uses the same protobuf structures
but accepts both binary protobuf and also JSON using jsonpb to encode and decode it.



### Major reduction of disk usage in neutrino db
The first version of neutrino db, the version from LND, used a combination of a database
and a couple of flat files. Unfortunately, synchronizing a db transaction with a flat file
write operation is incredibly difficult to do correctly, and the developers of LND
unfortunately did NOT do it correctly.

The result was data corruption when the wallet was hard stopped (by way of killing the
process or shutting off the device) and a lot of complex code for trying to manage
atomicity, without actually getting it right!

We deleted almost all of this code, switching to use of the neutrino.db database
completely which is based on bbolt database, a transactional key/value store inspired by
LMDB.

This solved the corruption problem, but the resulting database became quite large. The
neutrino db needs to be able to do a few things.

1. Get block header by block height OR block hash
2. Get filter header by block height OR block hash

The way we implemented all of these things was by creating a table of block header by hash,
filter header by hash, and hash by height. This worked but it used a lot of storage.
The hash was included three times as well as the block header and filter header.

In version 1.6.0 we replaced this schema with a new schema which stores block header and
filter header by height, and which stores height by 32 bit hash *prefix*. Because we only
store 32 bits of the hash, we accept that there may be a hash collision - i.e. multiple
block hashes which have the same first 32 bits. To resolve this, we store a *list* of
block heights for a given hash, and when doing a lookup, we check each one of the headers
and compute it's hash until we find the hash which matches. On average we will only have
a single entry in the list, but by being prepared for multiple entries we make the storage
size much smaller.

With these changes we were able to cut the neutrino.db file size roughly in half. But be
aware that when you update, the file will not get smaller because bboltdb is unable to
shrink the file, it can only create free space inside of the file. If you want the file
to be smaller, you will need to delete the neutrino.db file and allow pktwallet / pld to
re-create it.


## Minor changes

1. pld: Feature: Add pktdir param to pld - so you can specify the folder where the wallet is - f39d8d0dd8cf27f5ff0858abc0470b24e6903bdf
2. pktd: Bug: Fixed occasional crash caused by misuse of locks - 15c277ef8aa0c50d271fa68fb1650ce713bced84
3. pld: Bug: If some necessary directories don't exist, create them rather than crashing. 2a3949932b18a38ae5ca0deb35cc6a5567d3df5c 2a3949932b18a38ae5ca0deb35cc6a5567d3df5c
4. all: Improvement: Add github build process - c475471eff27c5a7963b5eccd13214c9fdf58013
5. pld: Bug: Fix how password is specified - 3af1c059a5716f8f0ffab86ed7cffa6c0fde14fb
6. pld: Bug: Private password must be verified when unlocking wallet or else there will be a crash later - 67e09e45328f59fca487e16443137d23d0446b99
7. all: Improvement: Remove non-working DNS seed nodes - 7d77cf273c25c43182dfff9ff657f2b629c93dc3
8. all: Improvement: Add seed nodes seed.pkt.ai and seed.pkt.pink - 281535f3b7b06261602e8edc97ee4325d84d0cc9 35ac19d2fb99d2f66c03b92e6082ab67b3c56677
9. pld: Improvement: Remove graceful shutdown so that the process will not try ot 
10. pld: Feature: Added ability to generate a wallet seed without creating the wallet.

## Stats
The pktd codebase unfortunately contains a fairly significant amount of *generated* code.
This exists because the tooling to generate it is too awkward to include in the build
scripts. I have separated the stats for generated code from the stats for hand-written
code.

Over this development period, a total of 215 files were changed, there were 82803 lines
of code altered or added. From these, 42 files and 46499 lines of generated code were
changed or added.

The size of the project increased by 5645 lines of written code, and 20373 lines of
generated code.

A total of 36304 lines, or 6.3% of the codebase, was manually altered.

pktd-v1.5.1 - 575425 lines of written code, 40319 lines of generated code
pktd-v1.6.0 - 581070 lines of written code, 60692 lines of generated code

## Future goals
In coming releases, we will be seeking to:
* Debug and stabilize Lightning Network
* Improve scalability
* Improve test coverage
* Move code generation to build time
* Reduce total hand-written code


## Thanks!
This release has received contributions from:
* Ivo Smits
* Aldebaran Perseke
* Dimitris Koukoulakis
* James P. Thomas
* Kyle Mason
