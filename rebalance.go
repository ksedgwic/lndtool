// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"
	
    "github.com/lightningnetwork/lnd/lnrpc"
    "github.com/lightningnetwork/lnd/lnrpc/routerrpc"
)

func hopPolicy(client lnrpc.LightningClient, ctx context.Context,
	chanId uint64, dstNode string) *lnrpc.RoutingPolicy {
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

var finalCLTVDelta = uint32(144)

func dumpRoute(client lnrpc.LightningClient, ctx context.Context,
	info *lnrpc.GetInfoResponse, route *lnrpc.Route) {

	fmt.Println("ChanId               Capacity     Amt    AmtMsat  Fee  FeeMsat Dlt PubKey                                                              FeeBase   FR  Dlt Alias")
	
	fmt.Printf("%29s %7d %10d %12s %4d %s %18s %s\n",
		"", 
		route.TotalAmt,
		route.TotalAmtMsat,
		"",
		route.TotalTimeLock - info.BlockHeight,
		info.IdentityPubkey,
		"",
		info.Alias,
	)

	// Make an array of the policies, one for each hop.
	policies := []*lnrpc.RoutingPolicy{}
	for _, hop := range route.Hops {
		policies = append(policies,
			hopPolicy(client, ctx, hop.ChanId, hop.PubKey))
	}
	
	for ndx, hop := range route.Hops {
		nodeInfo, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
			PubKey: hop.PubKey,
		})
		if err != nil {
			panic(fmt.Sprintf("GetNodeInfo failed[1]:", err))
		}
		alias := nodeInfo.Node.Alias

		// The policy information comes from the next hop.
		pstr := ""
		if ndx < len(route.Hops) - 1 {
			pstr = fmt.Sprintf("%7d %4d %4d",
				policies[ndx+1].FeeBaseMsat,
				policies[ndx+1].FeeRateMilliMsat,
				policies[ndx+1].TimeLockDelta,
			)
		} else {
			pstr = fmt.Sprintf("%7d %4d %4d",
				0,
				0,
				0,
			)
		}
		
		fmt.Printf("%d %10d %7d %10d %4d %7d %4d %s %18s %s\n",
			hop.ChanId,
			hop.ChanCapacity,
			hop.AmtToForward,
			hop.AmtToForwardMsat,
			hop.Fee,
			hop.FeeMsat,
			hop.Expiry - info.BlockHeight,
			hop.PubKey,
			pstr,
			alias,
		)
	}

	// Print fee totals.
	fmt.Printf("%48s %4d %7d\n",
		"",
		route.TotalFees,
		route.TotalFeesMsat,
	)
	fmt.Println()
}

func repriceRoute(
	client lnrpc.LightningClient, ctx context.Context,
	info *lnrpc.GetInfoResponse, route *lnrpc.Route, amt int64) {
	ll := len(route.Hops)
	
	sumDelta := finalCLTVDelta
	lastDelta := uint32(0)
	
	sumFeeMsat := int64(0)
	lastFeeMsat := int64(0)

	amtToFwdMsat := amt * 1000
	
	for ndx := ll - 1; ndx >= 0; ndx-- {
		hop := route.Hops[ndx]
		sndPolicy := hopPolicy(client, ctx, hop.ChanId, hop.PubKey)

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
	
	sumDelta := finalCLTVDelta
	lastDelta := uint32(0)
	
	sumFeeMsat := int64(0)
	lastFeeMsat := int64(0)
	
	for ndx := ll - 1; ndx >= 0; ndx-- {
		hop := route.Hops[ndx]
		sndPolicy := hopPolicy(client, ctx, hop.ChanId, hop.PubKey)

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

func rebalance(client lnrpc.LightningClient, router routerrpc.RouterClient, ctx context.Context, db *sql.DB, args []string) {
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

	doRebalance(client, router, ctx, db, amt, srcChanId, dstChanId, feeLimit)
}

func doRebalance(client lnrpc.LightningClient, router routerrpc.RouterClient, ctx context.Context, db *sql.DB, amt int64, srcChanId, dstChanId uint64, feeLimit float64) bool {
	
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

	// Defer creating invoice until we get far enough to need one.
	var invoiceRsp *lnrpc.AddInvoiceResponse = nil

	ourNode, err := hex.DecodeString(ourPubKey)
    if err != nil {
		panic(fmt.Sprintf("hex.DecodeString failed:", err))
    }
	
	for {
	RetryQuery:
		fmt.Printf("querying possible routes, ignoring %d edges\n",
			len(badEdges))
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
			IgnoredEdges: badEdges,
			IgnoredNodes: [][]byte{ ourNode },
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
			return false
		}

		// Only get one route, only consider the first slot.
		route := rsp.Routes[0]

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
			
		dumpRoute(client, ctx, info, route)

		checkRoute(client, ctx, info, route)

		// FIXME - REMOVE THIS
		os.Exit(0)
		
		if (route.TotalFeesMsat / 1000) > feeLimitFixed {
			fmt.Println("route exceeds fee limit")
			insertLoopAttempt(db, NewLoopAttempt(
				time.Now().Unix(),
				srcChanId, srcPubKey,
				dstChanId, dstPubKey,
				amt, feeLimit,
				LoopAttemptNoRoutes,
			))
			return false
		}

		ctxt, _ :=
			context.WithTimeout(context.Background(), time.Second * 60,)
		
		if invoiceRsp == nil {
			fmt.Println("generating invoice")
			
			// Generate an invoice.
			preimage := make([]byte, 32)
			_, err = rand.Read(preimage)
			if err != nil {
				panic(fmt.Sprintf("unable to generate preimage:", err))
			}
			invoice := &lnrpc.Invoice{
				Memo: fmt.Sprintf("rebalance %d %d %d",
					amt, srcChanId, dstChanId),
				RPreimage: preimage,
				Value:     amt,
			}
			invoiceRsp, err = client.AddInvoice(ctxt, invoice)
			if err != nil {
				panic(fmt.Sprintf("unable to add invoice:", err))
			}
		}

		fmt.Println("sending to route")

		sendRsp, err := router.SendToRoute(ctxt, &routerrpc.SendToRouteRequest{
			PaymentHash: invoiceRsp.RHash,
			Route: route,
		})
		if err != nil {
			panic(fmt.Sprintf("router.SendToRoute failed:", err))
		}

		if sendRsp.Failure != nil {
			pubKey :=
				hex.EncodeToString(sendRsp.Failure.GetFailureSourcePubkey())

			nodeInfo, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
				PubKey: pubKey,
			})
			if err != nil {
				panic(fmt.Sprintf("GetNodeInfo failed[1]:", err))
			}
			alias := nodeInfo.Node.Alias
			
			fmt.Printf("%30s: %s\n", alias, sendRsp.Failure.Code.String())
			fmt.Println()
			
			// Figure out which edge to ignore
			for ndx, hop := range route.Hops {
				if hop.PubKey == pubKey {
					// We want to drop the next hop.
					if ndx == len(route.Hops) - 1 {
						// Can't skip the last hop ... this one's done.
						fmt.Println("can't ignore last hop")
						goto FailedToRoute
					}
					chanId := route.Hops[ndx+1].ChanId
					nextChanInfo, err :=
						client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
						ChanId: chanId,
					})
					if err != nil {
						panic(fmt.Sprintf("hop GetChanInfo failed:", err))
					}
					reverse := nextChanInfo.Node2Pub == pubKey

					// Append this edge to the ignoredEdges and re-route.
					badEdges = append(badEdges, &lnrpc.EdgeLocator{
						ChannelId: chanId,
						DirectionReverse: reverse,

					})
					goto RetryQuery
				}
			}
			panic(fmt.Sprintf("couldn't find matching hop"))
		} else {
			fmt.Printf("PREIMAGE: %s\n", hex.EncodeToString(sendRsp.Preimage))
			insertLoopAttempt(db, NewLoopAttempt(
				time.Now().Unix(),
				srcChanId, srcPubKey,
				dstChanId, dstPubKey,
				amt, feeLimit,
				LoopAttemptSuccess,
			))
			return true
		}
	}

FailedToRoute:
	fmt.Println("failed to route payment")
	insertLoopAttempt(db, NewLoopAttempt(
		time.Now().Unix(),
		srcChanId, srcPubKey,
		dstChanId, dstPubKey,
		amt, feeLimit,
		LoopAttemptFailure,
	))
	return false
}
