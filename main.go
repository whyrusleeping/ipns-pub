package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"

	ma "gx/ipfs/QmR3JkmZBKYXgNMNsNZawm914455Qof3PEopwuVSeXG7aV/go-multiaddr"

	"github.com/ipfs/go-ipfs/namesys"
	path "github.com/ipfs/go-ipfs/path"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/routing/dht"
	"github.com/ipfs/go-ipfs/util/ipfsaddr"
	ci "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/crypto"
	"gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/host/basic"
	"gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/metrics"
	"gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/net/swarm"
	"gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/peer"

	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	//logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"

	cli "github.com/codegangsta/cli"
)

const IpnsValidatorTag = "ipns"

func getBootstrapAddrs() ([]ma.Multiaddr, error) {
	addrs := []string{
		"/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.236.151.122/tcp/4001/ipfs/QmSoLju6m7xTh3DuokvT3886QRYqxAzb1kShaanJgW36yx",
		"/ip4/104.236.176.52/tcp/4001/ipfs/QmSoLnSGccFuZQJzRadHn95W2CrSFmZuTdDWP8HXaHca9z",
		"/ip4/104.236.179.241/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
		"/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",
		"/ip4/128.199.219.111/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",
		"/ip4/162.243.248.213/tcp/4001/ipfs/QmSoLueR4xBeUbY9WZ9xGUUxunbKWcrNFTDAadQJmocnWm",
		"/ip4/178.62.158.247/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",
		"/ip4/178.62.61.185/tcp/4001/ipfs/QmSoLMeWqB7YGVLJN3pNLQpmmEk35v6wYtsMGLzSr5QBU3",
	}

	var maddrs []ma.Multiaddr
	for _, s := range addrs {
		a, err := ma.NewMultiaddr(s)
		if err != nil {
			return nil, err
		}

		maddrs = append(maddrs, a)
	}

	return maddrs, nil
}

func fatal(i interface{}) {
	fmt.Fprintln(os.Stderr, i)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "key",
			Usage: "specify file containing key to publish with",
		},
	}
	app.Action = pubFunc

	err := app.Run(os.Args)
	if err != nil {
		fatal(err)
	}
}

func loadKeyFile(fi string) (ci.PrivKey, error) {
	data, err := ioutil.ReadFile(fi)
	if err != nil {
		return nil, err
	}

	return ci.UnmarshalPrivateKey(data)
}

func pubFunc(c *cli.Context) {
	var priv ci.PrivKey
	if kf := c.String("key"); kf != "" {
		pk, err := loadKeyFile(kf)
		if err != nil {
			fatal(err)
		}

		priv = pk
	} else {
		fatal("must specify key file with '--key'")
	}

	if !c.Args().Present() {
		fatal("must specify path to publish")
	}

	p, err := path.ParsePath(c.Args().First())
	if err != nil {
		fatal(err)
	}

	bs, err := getBootstrapAddrs()
	if err != nil {
		fatal(err)
	}

	dstore := ds.NewMapDatastore()

	dht, err := spawnDHT(priv, dstore, bs)
	if err != nil {
		fatal(err)
	}

	err = pubPath(dht, priv, dstore, p)
	if err != nil {
		fatal(err)
	}
}

func spawnDHT(pk ci.PrivKey, dstore repo.Datastore, bootstraps []ma.Multiaddr) (*dht.IpfsDHT, error) {
	pub := pk.GetPublic()

	local, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	fmt.Println("Local peer ID: ", local.Pretty())

	ps := peer.NewPeerstore()
	ps.AddPrivKey(local, pk)
	ps.AddPubKey(local, pub)

	if len(bootstraps) > 0 {
		fmt.Println("Bootstrapping to:")
		for _, b := range bootstraps {
			fmt.Printf("  - %s\n", b)
		}
		fmt.Println()
	}

	listenaddr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		return nil, err
	}

	s, err := swarm.NewNetwork(context.Background(), []ma.Multiaddr{listenaddr}, local, ps, metrics.NewBandwidthCounter())
	if err != nil {
		fatal(err)
	}

	host := basichost.New(s)

	errs := make(chan error)
	for _, bsaddr := range bootstraps {
		go func(bsa ma.Multiaddr) {
			iaddr, err := ipfsaddr.ParseMultiaddr(bsa)
			if err != nil {
				fmt.Println("error parsing bootstrap: ", err)
				errs <- err
				return
			}

			ps.AddAddr(iaddr.ID(), iaddr.Transport(), peer.PermanentAddrTTL)

			ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
			err = host.Connect(ctx, ps.PeerInfo(iaddr.ID()))
			if err != nil {
				fmt.Printf("error connecting to peer: %s\n", err)
				errs <- err
				return
			}
			fmt.Printf("dial to %s succeeded!\n", iaddr.ID())
			errs <- nil
		}(bsaddr)
	}

	var good int
	for range bootstraps {
		err := <-errs
		if err == nil {
			good++
		}
	}
	if good == 0 {
		fatal("couldnt connect to any bootstrap peers")
	}

	idht := dht.NewDHT(context.Background(), host, dstore)
	idht.Validator[IpnsValidatorTag] = namesys.IpnsRecordValidator
	idht.Selector[IpnsValidatorTag] = namesys.IpnsSelectorFunc

	return idht, nil
}

func pubPath(dht *dht.IpfsDHT, privk ci.PrivKey, dstore repo.Datastore, p path.Path) error {
	pub := namesys.NewRoutingPublisher(dht, dstore)
	ctx := context.Background()
	return pub.Publish(ctx, privk, p)
}
