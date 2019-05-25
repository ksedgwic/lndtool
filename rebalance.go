// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strconv"
	"time"
	
    "github.com/lightningnetwork/lnd/lnrpc"
)

func rebalance(client lnrpc.LightningClient, ctx context.Context, args []string) {
	amt, err := strconv.Atoi(args[0])
	if err != nil {
		panic(fmt.Sprintf("failed to parse amount:", err))
	}
	srcChanId, err := strconv.Atoi(args[1])
	if err != nil {
		panic(fmt.Sprintf("failed to parse srcChanId:", err))
	}
	dstChanId, err := strconv.Atoi(args[2])
	if err != nil {
		panic(fmt.Sprintf("failed to parse dstChanId:", err))
	}

	// What is our own PubKey?
	info, err := client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
    if err != nil {
		panic(fmt.Sprintf("GetInfo failed[1]:", err))
    }
	ourPubKey := info.IdentityPubkey

	// What is the src pub key?
	srcChanInfo, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
		ChanId: uint64(srcChanId),
	})
    if err != nil {
		panic(fmt.Sprintf("src GetChanInfo failed:", err))
    }
	var srcPubKey string
	if srcChanInfo.Node1Pub == ourPubKey {
		srcPubKey = srcChanInfo.Node2Pub
	} else {
		srcPubKey = srcChanInfo.Node1Pub
	}
	
	// What is the dst pub key?
	dstChanInfo, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
		ChanId: uint64(dstChanId),
	})
    if err != nil {
		panic(fmt.Sprintf("dst GetChanInfo failed:", err))
    }
	var dstPubKey string
	if dstChanInfo.Node1Pub == ourPubKey {
		dstPubKey = dstChanInfo.Node2Pub
	} else {
		dstPubKey = dstChanInfo.Node1Pub
	}

	fmt.Printf("%d -> %s -> ... -> %s -> %d\n",
		srcChanId, srcPubKey, dstPubKey, dstChanId)

	feeLimitPercent := 0.01	// basis points
	feeLimitFixed := int64(float64(amt) * (feeLimitPercent / 100))
	fmt.Printf("using feeLimitFixed %d\n", feeLimitFixed)
	
	rsp, err := client.QueryRoutes(ctx, &lnrpc.QueryRoutesRequest {
		PubKey: dstPubKey,
		Amt: int64(amt),
		FeeLimit: &lnrpc.FeeLimit{
			Limit: &lnrpc.FeeLimit_Fixed{
				Fixed: feeLimitFixed,
			},
		},
		IgnoredNodes: [][]byte{},
		IgnoredEdges: []*lnrpc.EdgeLocator{{}},
		SourcePubKey: srcPubKey,
	})
    if err != nil {
		panic(fmt.Sprintf("QueryRoutes failed:", err))
    }

	// debug: dump the routes 
	for _, route := range rsp.Routes {
		fmt.Printf("%d\n", route.TotalFees)
		for _, hop := range route.Hops {
			fmt.Printf("%d %s %d\n", hop.ChanId, hop.PubKey, hop.Fee)
		}
	}

	os.Exit(0)
	
	// Generate an invoice.
	preimage := make([]byte, 32)
	_, err = rand.Read(preimage)
	if err != nil {
		panic(fmt.Sprintf("unable to generate preimage:", err))
	}
	invoice := &lnrpc.Invoice{
		Memo:      "rebalancing",
		RPreimage: preimage,
		Value:     int64(amt),
	}
	ctxt, _ := context.WithTimeout(
		context.Background(), time.Second * 30,
	)
	rsp2, err := client.AddInvoice(ctxt, invoice)
	if err != nil {
		panic(fmt.Sprintf("unable to add invoice:", err))
	}
	_ = rsp2
	
	// Just use the first route for now.

	// Add the first and last hops.
	
}
