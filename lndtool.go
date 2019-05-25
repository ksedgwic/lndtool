// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"flag"
    "fmt"
    "io/ioutil"
	"os"
    "os/user"
    "path"
	
    "github.com/lightningnetwork/lnd/lnrpc"
    "github.com/lightningnetwork/lnd/macaroons"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "gopkg.in/macaroon.v2"
)

func main() {

	flag.Parse()
	
    usr, err := user.Current()
    if err != nil {
        fmt.Println("Cannot get current user:", err)
        return
    }
    tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")
    macaroonPath := path.Join(usr.HomeDir,
		".lnd/data/chain/bitcoin/mainnet/admin.macaroon")

    tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
    if err != nil {
        fmt.Println("Cannot get node tls credentials", err)
        return
    }

    macaroonBytes, err := ioutil.ReadFile(macaroonPath)
    if err != nil {
        fmt.Println("Cannot read macaroon file", err)
        return
    }

    mac := &macaroon.Macaroon{}
    if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
        fmt.Println("Cannot unmarshal macaroon", err)
        return
    }

    opts := []grpc.DialOption{
        grpc.WithTransportCredentials(tlsCreds),
        grpc.WithBlock(),
        grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1 * 1024 * 1024 * 50)),
    }

    conn, err := grpc.Dial("localhost:10009", opts...)
    if err != nil {
        fmt.Println("cannot dial to lnd", err)
        return
    }
    client := lnrpc.NewLightningClient(conn)
	ctx := context.Background()

	cmd := flag.Args()[0]
	switch cmd {
	case "channels": { listChannels(client, ctx) }
	case "farside": { farSide(client, ctx) }
	case "rebalance": { rebalance(client, ctx, flag.Args()[1:4]) }
	default: {
        fmt.Printf("command \"%s\" unknown\n", cmd)
		os.Exit(1)
	}
	}
}
