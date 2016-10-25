# ipns-pub

A tool to publish ipns entries on the ipfs network with a given key

## Usage
First, you'll need to generate a publish key with [`ipfs-key`](https://github.com/whyrusleeping/ipfs-key).
Once you have one, simply:
```
ipns-pub -key=mykeyfile /ipfs/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC
```

The program will start up an ipfs dht, bootstrap to the network, and publish the given
ipfs path to the ipns entry of your given key.

## Installation
```
go get -d github.com/ipfs/go-ipfs
cd $GOPATH/src/github.com/ipfs/go-ipfs
git checkout 4fa85cc
make deps

go get -d github.com/whyrusleeping/ipns-pub
cd $GOPATH/src/github.com/whyrusleeping/ipns-pub
gx --verbose install --global
go install
```

Note: depends on gx, install with:
```
go get -u github.com/whyrusleeping/gx
go get -u github.com/whyrusleeping/gx-go
```

### License
MIT
