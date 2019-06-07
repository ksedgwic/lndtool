// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
    "fmt"
    "io/ioutil"
	"os"
	
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
	
	cfg, args, err := loadConfig()
	if err != nil {
		panic(fmt.Sprintf("loadConfig failed: %v", err))
	}

    tlsCreds, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
    if err != nil {
        fmt.Println("Cannot get node tls credentials", err)
        return
    }

    macaroonBytes, err := ioutil.ReadFile(cfg.MacaroonPath)
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

    conn, err := grpc.Dial(cfg.RPCServer, opts...)
    if err != nil {
        fmt.Println("cannot dial to lnd", err)
        return
    }
    client := lnrpc.NewLightningClient(conn)
	router := routerrpc.NewRouterClient(conn)
	ctx := context.Background()

	db := openDatabase()

	if len(args) == 0 {
		// TODO - generate usage message here
		for _, val := range cfg.Recommend.PeerNodeBlacklist {
			fmt.Println(val)
		}
		os.Exit(0)
	}
	
	cmd := args[0]
	switch cmd {
	case "channels": { listChannels(cfg, client, ctx, db) }
	case "farside": { farSide(cfg, client, ctx) }
	case "rebalance": { rebalance(cfg, client, router, ctx, db, args[1:]) }
	case "recommend": { recommend(cfg, client, router, ctx, db, args[1:]) }
	case "autobalance": {
		for {
			if !recommend(cfg, client, router, ctx, db, args[1:]) {
				break
			}
		}
	}
	case "mkdb": { createDatabase(cfg, db) }
	default: {
        fmt.Printf("command \"%s\" unknown\n", cmd)
		os.Exit(1)
	}
	}
}
