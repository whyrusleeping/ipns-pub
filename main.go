package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"gx/ipfs/QmPGpCi2W5SuzbYJE3NwuMtb95MPataUxuejYW1NLNr6sj/go-ipfs/namesys"
	path "gx/ipfs/QmPGpCi2W5SuzbYJE3NwuMtb95MPataUxuejYW1NLNr6sj/go-ipfs/path"
	"gx/ipfs/QmPGpCi2W5SuzbYJE3NwuMtb95MPataUxuejYW1NLNr6sj/go-ipfs/repo"
	"gx/ipfs/QmPGpCi2W5SuzbYJE3NwuMtb95MPataUxuejYW1NLNr6sj/go-ipfs/thirdparty/ipfsaddr"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	cli "gx/ipfs/QmVcLF2CgjQb5BWmYFWsDfxDjbzBfcChfdHRedxeL3dV4K/cli"
	"gx/ipfs/QmWHiyk5y2EKgxHogFJ4Zt1xTqKeVsBc4zcBke8ie9C2Bn/go-libp2p-kad-dht"
	"gx/ipfs/QmWpTXhTkpoCDEm9twJd5Rc9jFwy61emzxneeJzrVMfjGF/go-libp2p-metrics"
	pstore "gx/ipfs/QmXXCcQ7CLg5a81Ui9TTR35QcR4y7ZyihxwfjqaHfUVcVo/go-libp2p-peerstore"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	"gx/ipfs/QmcRa2qn6iCmap9bjp8jAwkvYAq13AUfxdY3rrYiaJbLum/go-libp2p/p2p/host/basic"
	"gx/ipfs/QmeAfPWBWDQq9qjQ5oiWhaFs7oEsfB6FyEj5VxNdc2r34q/go-libp2p-swarm"
	"gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	ci "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
)

const IpnsValidatorTag = "ipns"

func fatal(i interface{}) {
	fmt.Fprintln(os.Stderr, i)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "ipns-pub"
	app.Usage = `ipns-pub can be used to publish an ipns entry with a given private key

Example:

    ipns-pub -key=mykeyfile /ipfs/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC

Key Generation:

To generate a key, install ipfs-key:

    go get github.com/whyrusleeping/ipfs-key

And then run:

    ipfs-key > mykeyfile
`
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "key",
			Usage: "specify file containing key to publish with",
		},
		cli.BoolFlag{
			Name:  "daemon",
			Usage: "run a daemon that republishes the given ipns entry",
		},
		cli.StringFlag{
			Name:  "interval",
			Usage: "specify the republish interval for daemon mode (default 12h)",
		},
	}
	app.Action = pubFunc

	err := app.Run(os.Args)
	if err != nil {
		fatal(err)
	}
}

func pubFunc(c *cli.Context) error {
	var priv ci.PrivKey
	if kf := c.String("key"); kf != "" {
		pk, err := loadKeyFile(kf)
		if err != nil {
			return err
		}

		priv = pk
	} else {
		return fmt.Errorf("must specify key file with '--key'")
	}

	if !c.Args().Present() {
		return fmt.Errorf("must specify path to publish")
	}

	p, err := path.ParsePath(c.Args().First())
	if err != nil {
		return err
	}

	interv := c.String("interval")
	var ticktime time.Duration = time.Hour * 12
	if interv != "" {
		d, err := time.ParseDuration(interv)
		if err != nil {
			return err
		}

		ticktime = d
	}

	bs, err := getBootstrapAddrs()
	if err != nil {
		return err
	}

	dstore := ds.NewMapDatastore()

	dht, err := spawnDHT(priv, dstore, bs)
	if err != nil {
		return err
	}

	nsys := namesys.NewRoutingPublisher(dht, dstore)

	// publish once in either case
	err = nsys.Publish(context.TODO(), priv, p)
	if err != nil {
		return err
	}

	if !c.Bool("daemon") {
		return nil
	}

	for range time.Tick(ticktime) {
		fmt.Println("publishing...")
		before := time.Now()
		err = nsys.Publish(context.TODO(), priv, p)
		if err != nil {
			// TODO: probably don't want to actually error out and die here
			return err
		}
		fmt.Println("publish took: ", time.Now().Sub(before))
	}
	return nil
}

// currently just uses the ipfs core bootstrap nodes
// TODO: allow user to specify their own
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

func loadKeyFile(fi string) (ci.PrivKey, error) {
	data, err := ioutil.ReadFile(fi)
	if err != nil {
		return nil, err
	}

	return ci.UnmarshalPrivateKey(data)
}

func spawnDHT(pk ci.PrivKey, dstore repo.Datastore, bootstraps []ma.Multiaddr) (*dht.IpfsDHT, error) {
	pub := pk.GetPublic()

	local, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	fmt.Println("Local peer ID: ", local.Pretty())

	ps := pstore.NewPeerstore()
	ps.AddPrivKey(local, pk)
	ps.AddPubKey(local, pub)

	listenaddr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		return nil, err
	}

	s, err := swarm.NewNetwork(context.Background(), []ma.Multiaddr{listenaddr}, local, ps, metrics.NewBandwidthCounter())
	if err != nil {
		fatal(err)
	}

	host := basichost.New(s)

	idht := dht.NewDHT(context.Background(), host, dstore)
	idht.Validator[IpnsValidatorTag] = namesys.IpnsRecordValidator
	idht.Selector[IpnsValidatorTag] = namesys.IpnsSelectorFunc

	err = doBootstrap(host, idht, bootstraps)
	if err != nil {
		return nil, err
	}

	return idht, nil
}

func doBootstrap(bh *basichost.BasicHost, idht *dht.IpfsDHT, bootstraps []ma.Multiaddr) error {
	if len(bootstraps) > 0 {
		fmt.Println("Bootstrapping to:")
		for _, b := range bootstraps {
			fmt.Printf("  - %s\n", b)
		}
		fmt.Println()
	}

	errs := make(chan error)
	for _, bsaddr := range bootstraps {
		go func(bsa ma.Multiaddr) {
			iaddr, err := ipfsaddr.ParseMultiaddr(bsa)
			if err != nil {
				fmt.Println("error parsing bootstrap: ", err)
				errs <- err
				return
			}

			bh.Peerstore().AddAddr(iaddr.ID(), iaddr.Transport(), pstore.PermanentAddrTTL)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			pi := bh.Peerstore().PeerInfo(iaddr.ID())
			err = bh.Connect(ctx, pi)
			if err != nil {
				fmt.Printf("error connecting to peer: %s\n", err)
				errs <- err
				return
			}

			fmt.Printf("dial to %s succeeded!\n", iaddr.ID())

			idht.Update(context.TODO(), iaddr.ID())
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
		return fmt.Errorf("couldnt connect to any bootstrap peers")
	}

	return nil
}
