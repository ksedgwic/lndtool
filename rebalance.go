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

func hopPolicy(client lnrpc.LightningClient, ctx context.Context, chanId uint64, dstNode string) *lnrpc.RoutingPolicy {
	chanInfo, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{ChanId: chanId})
	if err != nil {
		panic(fmt.Sprintf("last GetChanInfo failed:", err))
	}
	if chanInfo.Node1Pub == dstNode {
		return chanInfo.Node2Policy
	} else {
		return chanInfo.Node1Policy
	}
}

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
		SourcePubKey: srcPubKey,
	})
    if err != nil {
		panic(fmt.Sprintf("QueryRoutes failed:", err))
    }

	// debug: dump the routes 
	for _, route := range rsp.Routes {
		fmt.Println()
		for ndx, hop := range route.Hops {
			// Is this the last hop in the partial route?
			policy := hopPolicy(client, ctx, hop.ChanId, hop.PubKey)
			if ndx == len(route.Hops) - 1 {
				// Add in the fees for the last hop (since we are adding another)
				hop.Fee = (policy.FeeBaseMsat +
					(policy.FeeRateMilliMsat * int64(amt) / 1000)) / 1000
				route.TotalFees += hop.Fee
			}
			fmt.Printf("%d %s %6d %5d\n", hop.ChanId, hop.PubKey, hop.Fee, policy.TimeLockDelta)
		}
		fmt.Printf("                                                                                      %6d %5d\n", route.TotalFees, route.TotalTimeLock - info.BlockHeight)
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
