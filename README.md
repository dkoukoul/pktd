# pktd

[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://Copyfree.org)
![Master branch build Status](https://github.com/pkt-cash/pktd/actions/workflows/go.yml/badge.svg?branch=master)
![Develop branch build Status](https://github.com/pkt-cash/pktd/actions/workflows/go.yml/badge.svg?branch=develop)

This repository contains **pktd**, **pktwallet**, and **pld**, the PKT
Lightning Daemon.

`pktd` is the primary mainnet node software for the PKT blockchain.
It is known to correctly download, validate, and serve the chain,
using rules for block acceptance based on Bitcoin Core, with the
addition of PacketCrypt Proofs. 

`pktwallet` is the **old** wallet software which is used for managing
PKT coins. It does not support Lightning Network and it will be phased
out over time.

`pld` is the PKT Lightning Daemon, lightning network enabled wallet.
All new effort is focused on pld and this will be the main wallet going
forward.

`pktctl` is a management app for controlling pktd and pktwallet.

`pldctl` is a management app for controlling pld.

## Requirements

* [Golang compiler](https://command-not-found.com/go)
* [Protocol buffers](https://command-not-found.com/protoc)

## Issue Tracker

* The GitHub [integrated GitHub issue tracker](https://github.com/pkt-cash/pktd/issues) is used for this project.  

## Building

Using `git`, clone the project from the repository:

```bash
$ git clone https://github.com/pkt-cash/pktd
$ cd pktd
$ ./do
```

This will build `pktd`, `pktwallet`, `pktctl`, `pld`, and `pldctl` inside of the `./bin` sub-folder.

### Advanced building

The `./do` build script invokes golang code to build golang code, so using normal environment
variables will affect the build code as well as the final code. To use environment variables and
affect the final code without the build code, place them *after* the `./do` command, not before.


Cross-compiling for windows on a Mac:

```bash
$ ./do GOOS=windows GOARCH=amd64
```

The script will only accept env vars if they begin with CAPITAL letters, numbers and the underscore
before an equal sign. So `MY_ENV_VAR=value` will be passed through as an environment variable, but
`my_env_var=value` will not.

Whatever does not match the env var pattern is treated as a command line flag for the go build.
For example `./do -tags dev` will run `go build` with `-tags dev` argument.

Finally, you can run the build manually, but you must run ./do first because some code is generated.
But generated code does not depend on the OS or architecture, so you can safely compile using
whatever tool you prefer, after you have run `./do` once.

## Testing

To run the tests, run `./test.sh`. Each test will be run individually and the output will be written
to a file inside of the folder `./testout`.

## Documentation

The documentation for `pktd` is available in the [docs.pkt.cash](https://docs.pkt.cash) site.

## License

`pktd` is licensed under the [Copyfree](http://Copyfree.org) ISC License.
