// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"
	
    "github.com/lightningnetwork/lnd/lnrpc"
)

// When true, don't add the first and last hops to the routes and quit
// after displaying the lnd priced route.
//
var feeDebug = false

func hopPolicy(client lnrpc.LightningClient, ctx context.Context,
	chanId uint64, dstNode string) (*lnrpc.RoutingPolicy, *lnrpc.RoutingPolicy) {
	chanInfo, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{ChanId: chanId})
	if err != nil {
		panic(fmt.Sprintf("last GetChanInfo failed:", err))
	}
	if chanInfo.Node1Pub == dstNode {
		return chanInfo.Node2Policy, chanInfo.Node1Policy
	} else {
		return chanInfo.Node1Policy, chanInfo.Node2Policy
	}
}

func hopStr(info *lnrpc.GetInfoResponse, hop *lnrpc.Hop) string {
	return fmt.Sprintf("%d %10d %7d %10d %4d %7d %4d %s",
		hop.ChanId,
		hop.ChanCapacity,
		hop.AmtToForward,
		hop.AmtToForwardMsat,
		hop.Fee,
		hop.FeeMsat,
		hop.Expiry - info.BlockHeight,
		hop.PubKey,
	)
}

func policyStr(info *lnrpc.GetInfoResponse,
	sndPolicy *lnrpc.RoutingPolicy, rcvPolicy *lnrpc.RoutingPolicy) string {
	return fmt.Sprintf("%7d %4d %4d",
		sndPolicy.FeeBaseMsat,
		sndPolicy.FeeRateMilliMsat,
		sndPolicy.TimeLockDelta,
		// rcvPolicy.FeeBaseMsat,
		// rcvPolicy.FeeRateMilliMsat,
		// rcvPolicy.TimeLockDelta,
	)
}

func routeStr(info *lnrpc.GetInfoResponse, route *lnrpc.Route) string {
	return fmt.Sprintf("%29s %7d %10d %4d %7d %4d",
		"",
		route.TotalAmt,
		route.TotalAmtMsat,
		route.TotalFees,
		route.TotalFeesMsat,
		route.TotalTimeLock - info.BlockHeight,
	)
}

func dumpRoute(client lnrpc.LightningClient, ctx context.Context,
	info *lnrpc.GetInfoResponse, route *lnrpc.Route) {
	for ndx, hop := range route.Hops {

		nodeInfo, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
			PubKey: hop.PubKey,
		})
		if err != nil {
			panic(fmt.Sprintf("GetNodeInfo failed[1]:", err))
		}
		alias := nodeInfo.Node.Alias

		sndPolicy, rcvPolicy :=
			hopPolicy(client, ctx, hop.ChanId, hop.PubKey)

		pstr := "                 "
		if ndx != 0 {
			pstr = policyStr(info, sndPolicy, rcvPolicy)
		}
		
		fmt.Printf("%s %s %s\n", hopStr(info, hop), pstr, alias)
	}
	fmt.Printf("%s\n", routeStr(info, route))
	fmt.Println()
}

var finalCLTVDelta = uint32(144)

func repriceRoute(
	client lnrpc.LightningClient, ctx context.Context,
	info *lnrpc.GetInfoResponse, route *lnrpc.Route, amt int64) {
	ll := len(route.Hops)
	
	sumDelta := uint32(finalCLTVDelta)
	lastDelta := uint32(0)
	
	sumFeeMsat := int64(0)
	lastFeeMsat := int64(0)

	amtToFwdMsat := amt * 1000
	
	for ndx := ll - 1; ndx >= 0; ndx-- {
		hop := route.Hops[ndx]
		sndPolicy, _ := hopPolicy(client, ctx, hop.ChanId, hop.PubKey)

		hop.Expiry = info.BlockHeight + sumDelta
		
		if ndx != ll - 1 {
			sumDelta += lastDelta
		}
		
		lastDelta = sndPolicy.TimeLockDelta

		hop.FeeMsat = lastFeeMsat
		hop.Fee = lastFeeMsat / 1000
		hop.AmtToForwardMsat = amtToFwdMsat
		hop.AmtToForward = amtToFwdMsat / 1000

		amtToFwdMsat += lastFeeMsat
		sumFeeMsat += lastFeeMsat
		
		lastFeeMsat =
			sndPolicy.FeeBaseMsat +
			(hop.AmtToForwardMsat * sndPolicy.FeeRateMilliMsat) / 1000000
	}
	
	route.TotalTimeLock = info.BlockHeight + sumDelta
	route.TotalFeesMsat = sumFeeMsat
	route.TotalFees = sumFeeMsat / 1000
	route.TotalAmtMsat = (amt * 1000) + sumFeeMsat
	route.TotalAmt = ((amt * 1000) + sumFeeMsat) / 1000
}

func checkRoute(client lnrpc.LightningClient, ctx context.Context,
	info *lnrpc.GetInfoResponse, route *lnrpc.Route) {
	ll := len(route.Hops)
	
	sumDelta := uint32(finalCLTVDelta)
	lastDelta := uint32(0)
	
	sumFeeMsat := int64(0)
	lastFeeMsat := int64(0)
	
	for ndx := ll - 1; ndx >= 0; ndx-- {
		hop := route.Hops[ndx]
		sndPolicy, _ := hopPolicy(client, ctx, hop.ChanId, hop.PubKey)

		if hop.Expiry - info.BlockHeight != sumDelta {
			panic(fmt.Sprintf("bad expiry on hop %d", ndx))
		}
		
		if ndx != ll - 1 {
			sumDelta += lastDelta
		}

		lastDelta = sndPolicy.TimeLockDelta

		if hop.FeeMsat != lastFeeMsat {
			panic(fmt.Sprintf("bad fee on hop %d, saw %d, expected %d",
				ndx, lastFeeMsat, hop.FeeMsat))
		}
		sumFeeMsat += lastFeeMsat
		
		lastFeeMsat =
			sndPolicy.FeeBaseMsat +
			(hop.AmtToForwardMsat * sndPolicy.FeeRateMilliMsat) / 1000000
	}
	if route.TotalTimeLock - info.BlockHeight != sumDelta {
		panic(fmt.Sprintf("bad route total"))
	}
	if route.TotalFeesMsat != sumFeeMsat {
		panic(fmt.Sprintf("bad fee total"))
	}
}

func rebalance(client lnrpc.LightningClient, ctx context.Context, db *sql.DB,
	args []string) {
	amti, err := strconv.Atoi(args[0])
	if err != nil {
		panic(fmt.Sprintf("failed to parse amount:", err))
	}
	amt := int64(amti)
	
	srcChanIdI, err := strconv.Atoi(args[1])
	if err != nil {
		panic(fmt.Sprintf("failed to parse srcChanId:", err))
	}
	srcChanId := uint64(srcChanIdI)
	
	dstChanIdI, err := strconv.Atoi(args[2])
	if err != nil {
		panic(fmt.Sprintf("failed to parse dstChanId:", err))
	}
	dstChanId := uint64(dstChanIdI)

	feeLimit := 0.0001   // one basis point default
	if len(args) > 3 {
		feeLimit, err = strconv.ParseFloat(args[3], 64)
		if err != nil {
			panic(fmt.Sprintf("failed to parse feeLimit:", err))
		}
	}
	
	// What is our own PubKey?
	info, err := client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
    if err != nil {
		panic(fmt.Sprintf("GetInfo failed[1]:", err))
    }
	ourPubKey := info.IdentityPubkey

	// What is the src pub key?
	srcChanInfo, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
		ChanId: srcChanId,
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
		ChanId: dstChanId,
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

	feeLimitPercent := feeLimit * 100
	feeLimitFixed := int64(float64(amt) * (feeLimitPercent / 100))
	fmt.Printf("limit fee rate to %f, %d sat\n", feeLimit, feeLimitFixed)

	fmt.Println("querying possible routes")
	rsp, err := client.QueryRoutes(ctx, &lnrpc.QueryRoutesRequest {
		PubKey: dstPubKey,
		Amt: amt,
		FeeLimit: &lnrpc.FeeLimit{
			Limit: &lnrpc.FeeLimit_Fixed{
				Fixed: feeLimitFixed,
			},
		},
		SourcePubKey: srcPubKey,
		FinalCltvDelta: int32(finalCLTVDelta),
	})
    if err != nil {
		fmt.Println("no routes found at this fee limit")
		insertLoopAttempt(db, NewLoopAttempt(
			time.Now().Unix(),
			srcChanId, srcPubKey,
			dstChanId, dstPubKey,
			amt, feeLimit,
			LoopAttemptNoRoutes,
		))
		os.Exit(1)
    }

	economicRoutes := []*lnrpc.Route{}
	for _, route := range rsp.Routes {

		if !feeDebug {
			// Prepend the initial hop from us through the src channel
			hop0 := &lnrpc.Hop{
				ChanId: srcChanId,
				ChanCapacity: srcChanInfo.Capacity,
				AmtToForward: amt,
				PubKey: srcPubKey,
				// We will set all of these when we "reprice" the route.
				// Fee:
				// Expiry:
				// AmtToForwardMsat:
				// FeeMSat:
			}
			route.Hops = append([]*lnrpc.Hop{ hop0 }, route.Hops...)

			// Append the final hop back to us through the dst channel
			hopN := &lnrpc.Hop{
				ChanId: dstChanId,
				ChanCapacity: dstChanInfo.Capacity,
				AmtToForward: amt,
				PubKey: ourPubKey,
				// We will set all of these when we "reprice" the route.
				// Fee:
				// Expiry:
				// AmtToForwardMsat:
				// FeeMSat:
			}
			route.Hops = append(route.Hops, hopN)
			
			repriceRoute(client, ctx, info, route, amt)
		}
		
		dumpRoute(client, ctx, info, route)

		checkRoute(client, ctx, info, route)

		if !feeDebug {
			if (route.TotalFeesMsat / 1000) <= feeLimitFixed {
				economicRoutes = append(economicRoutes, route)
			}
		}
	}

	if len(economicRoutes) == 0 {
		fmt.Println("no routes inside this fee limit")
		insertLoopAttempt(db, NewLoopAttempt(
			time.Now().Unix(),
			srcChanId, srcPubKey,
			dstChanId, dstPubKey,
			amt, feeLimit,
			LoopAttemptNoRoutes,
		))
		os.Exit(1)
	} else {
		fmt.Printf("found %d economic routes\n", len(economicRoutes))
	}
	
	fmt.Println("generating invoice")
	
	// Generate an invoice.
	preimage := make([]byte, 32)
	_, err = rand.Read(preimage)
	if err != nil {
		panic(fmt.Sprintf("unable to generate preimage:", err))
	}
	invoice := &lnrpc.Invoice{
		Memo: fmt.Sprintf("rebalance %d %s %s", args[0], args[1], args[2]),
		RPreimage: preimage,
		Value:     amt,
	}
	ctxt, _ := context.WithTimeout(context.Background(), time.Second * 60,)
	invoiceRsp, err := client.AddInvoice(ctxt, invoice)
	if err != nil {
		panic(fmt.Sprintf("unable to add invoice:", err))
	}

	for _, route := range(economicRoutes) {

		fmt.Println("TRYING:")
		dumpRoute(client, ctx, info, route)

		req := &lnrpc.SendToRouteRequest{
			PaymentHash: invoiceRsp.RHash,
			Route: route,
		}

		stream, err := client.SendToRoute(ctxt)
		if err != nil {
			fmt.Printf("client.SendToRoute failed:", err)
			continue
		}

		if err := stream.Send(req); err != nil {
			fmt.Printf("stream.Send failed:", err)
			continue
		}

		sendRsp, err := stream.Recv()
		if err != nil {
			fmt.Printf("stream.Recv failed:", err)
			continue
		}
		

		if sendRsp.PaymentError == "" {
			fmt.Println(sendRsp)
			insertLoopAttempt(db, NewLoopAttempt(
				time.Now().Unix(),
				srcChanId, srcPubKey,
				dstChanId, dstPubKey,
				amt, feeLimit,
				LoopAttemptSuccess,
			))
			os.Exit(0)
		} else {
			fmt.Printf("PaymentError: %v\n", sendRsp.PaymentError)
			fmt.Println()
		}
		
		// time.Sleep(1 * time.Second)
	}

	fmt.Println("failed to route payment")
	insertLoopAttempt(db, NewLoopAttempt(
		time.Now().Unix(),
		srcChanId, srcPubKey,
		dstChanId, dstPubKey,
		amt, feeLimit,
		LoopAttemptFailure,
	))
	os.Exit(1)
}
