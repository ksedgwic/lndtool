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
    "github.com/lightningnetwork/lnd/lnrpc/routerrpc"
    "github.com/lightningnetwork/lnd/macaroons"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "gopkg.in/macaroon.v2"
)

// Keep accumulating bad edges.
var edgeLimit map[*lnrpc.EdgeLocator]int64

func main() {

	edgeLimit = map[*lnrpc.EdgeLocator]int64{}
	
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
	router := routerrpc.NewRouterClient(conn)
	ctx := context.Background()

	db := openDatabase()

	cmd := flag.Args()[0]
	switch cmd {
	case "channels": { listChannels(client, ctx, db) }
	case "farside": { farSide(client, ctx) }
	case "rebalance": { rebalance(client, router, ctx, db, flag.Args()[1:]) }
	case "recommend": { recommend(client, router, ctx, db, flag.Args()[1:]) }
	case "autobalance": {
		for {
			if !recommend(client, router, ctx, db, flag.Args()[1:]) {
				break
			}
		}
	}
	case "mkdb": { createDatabase(db) }
	default: {
        fmt.Printf("command \"%s\" unknown\n", cmd)
		os.Exit(1)
	}
	}
}
