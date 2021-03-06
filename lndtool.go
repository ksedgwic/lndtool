// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"database/sql"
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

// TODO - can this be moved to recommend or rebalance?
// Keep accumulating bad edges.
var edgeLimit map[*lnrpc.EdgeLocator]int64

var (
	gCfg    *config
	gClient lnrpc.LightningClient
	gRouter routerrpc.RouterClient
	gCtx    context.Context
	gDB     *sql.DB
)

func main() {
	edgeLimit = map[*lnrpc.EdgeLocator]int64{}

	var err error
	gCfg, err = loadConfig()
	if err != nil {
		os.Exit(0)
	}

	tlsCreds, err := credentials.NewClientTLSFromFile(gCfg.TLSCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
		return
	}

	macaroonBytes, err := ioutil.ReadFile(gCfg.MacaroonPath)
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

	conn, err := grpc.Dial(gCfg.RPCServer, opts...)
	if err != nil {
		fmt.Println("cannot dial to lnd", err)
		return
	}
	gClient = lnrpc.NewLightningClient(conn)
	gRouter = routerrpc.NewRouterClient(conn)
	gCtx = context.Background()

	openDatabase()
	createDatabase()

	if command != nil {
		command.RunCommand()
		os.Exit(0)
	}
}
