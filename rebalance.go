// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
)

var ignoreBadEdges = true // Ignore bad edges on subsequent QueryRoutes

func hopPolicy(chanId uint64, dstNode string) *lnrpc.RoutingPolicy {
	chanInfo, err :=
		gClient.GetChanInfo(gCtx, &lnrpc.ChanInfoRequest{ChanId: chanId})
	if err != nil {
		panic(fmt.Sprintf("last GetChanInfo failed:", err))
	}
	if chanInfo.Node1Pub == dstNode {
		return chanInfo.Node2Policy
	} else {
		return chanInfo.Node1Policy
	}
}

func dumpRoute(info *lnrpc.GetInfoResponse, route *lnrpc.Route) {

	fmt.Println("ChanId               Capacity     Amt    AmtMsat  Fee  FeeMsat Dlt PubKey                                                                   FB   FR  Dlt Alias")

	fmt.Printf("%29s %7d %10d %12s %4d %s %18s %s\n",
		"",
		route.TotalAmt,
		route.TotalAmtMsat,
		"",
		route.TotalTimeLock-info.BlockHeight,
		info.IdentityPubkey,
		"",
		info.Alias,
	)

	// Make an array of the policies, one for each hop.
	policies := []*lnrpc.RoutingPolicy{}
	for _, hop := range route.Hops {
		policies = append(policies,
			hopPolicy(hop.ChanId, hop.PubKey))
	}

	for ndx, hop := range route.Hops {
		nodeInfo, err := gClient.GetNodeInfo(gCtx, &lnrpc.NodeInfoRequest{
			PubKey: hop.PubKey,
		})
		if err != nil {
			panic(fmt.Sprintf("GetNodeInfo failed[1]:", err))
		}
		alias := nodeInfo.Node.Alias

		// The policy information comes from the next hop.
		pstr := ""
		if ndx < len(route.Hops)-1 {
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
			hop.Expiry-info.BlockHeight,
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

func repriceRoute(info *lnrpc.GetInfoResponse, route *lnrpc.Route, amt int64) {
	ll := len(route.Hops)

	sumDelta := gCfg.Rebalance.FinalCLTVDelta
	lastDelta := uint32(0)

	sumFeeMsat := int64(0)
	lastFeeMsat := int64(0)

	amtToFwdMsat := amt * 1000

	for ndx := ll - 1; ndx >= 0; ndx-- {
		hop := route.Hops[ndx]
		sndPolicy := hopPolicy(hop.ChanId, hop.PubKey)

		hop.Expiry = info.BlockHeight + sumDelta

		if ndx != ll-1 {
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
				(hop.AmtToForwardMsat*sndPolicy.FeeRateMilliMsat)/1000000
	}

	route.TotalTimeLock = info.BlockHeight + sumDelta
	route.TotalFeesMsat = sumFeeMsat
	route.TotalFees = sumFeeMsat / 1000
	route.TotalAmtMsat = (amt * 1000) + sumFeeMsat
	route.TotalAmt = ((amt * 1000) + sumFeeMsat) / 1000
}

func checkRoute(info *lnrpc.GetInfoResponse, route *lnrpc.Route) {
	ll := len(route.Hops)

	sumDelta := gCfg.Rebalance.FinalCLTVDelta
	lastDelta := uint32(0)

	sumFeeMsat := int64(0)
	lastFeeMsat := int64(0)

	for ndx := ll - 1; ndx >= 0; ndx-- {
		hop := route.Hops[ndx]
		sndPolicy := hopPolicy(hop.ChanId, hop.PubKey)

		if hop.Expiry-info.BlockHeight != sumDelta {
			panic(fmt.Sprintf("bad expiry on hop %d", ndx))
		}

		if ndx != ll-1 {
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
				(hop.AmtToForwardMsat*sndPolicy.FeeRateMilliMsat)/1000000
	}
	if route.TotalTimeLock-info.BlockHeight != sumDelta {
		panic(fmt.Sprintf("bad route total"))
	}
	if route.TotalFeesMsat != sumFeeMsat {
		panic(fmt.Sprintf("bad fee total"))
	}
}

func rebalance(args []string) {
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

	doRebalance(amt, srcChanId, dstChanId)
}

func doRebalance(amt int64, srcChanId, dstChanId uint64) bool {

	// What is our own PubKey?
	info, err := gClient.GetInfo(gCtx, &lnrpc.GetInfoRequest{})
	if err != nil {
		panic(fmt.Sprintf("GetInfo failed[1]:", err))
	}
	ourPubKey := info.IdentityPubkey

	// What is the src pub key?
	srcChanInfo, err := gClient.GetChanInfo(gCtx, &lnrpc.ChanInfoRequest{
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
	srcNodeInfo, err := gClient.GetNodeInfo(gCtx, &lnrpc.NodeInfoRequest{
		PubKey: srcPubKey,
	})
	if err != nil {
		panic(fmt.Sprint("src GetNodeInfo failed:", err))
	}

	// What is the dst pub key?
	dstChanInfo, err := gClient.GetChanInfo(gCtx, &lnrpc.ChanInfoRequest{
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
	dstNodeInfo, err := gClient.GetNodeInfo(gCtx, &lnrpc.NodeInfoRequest{
		PubKey: dstPubKey,
	})
	if err != nil {
		panic(fmt.Sprint("dst GetNodeInfo failed:", err))
	}

	feeLimitPercent := gCfg.Rebalance.FeeLimitRate * 100
	feeLimitFixed := int64(float64(amt) * (feeLimitPercent / 100))
	if gCfg.Verbose {
		fmt.Printf("limit fee rate to %f, %d sat\n",
			gCfg.Rebalance.FeeLimitRate, feeLimitFixed)
	}

	// Defer creating invoice until we get far enough to need one.
	var invoiceRsp *lnrpc.AddInvoiceResponse = nil

	ourNode, err := hex.DecodeString(ourPubKey)
	if err != nil {
		panic(fmt.Sprintf("hex.DecodeString failed:", err))
	}

	for {
	RetryQuery:
		badEdges := []*lnrpc.EdgeLocator{}
		if ignoreBadEdges {
			// Reject all edges that are known to fail at this amount.
			for edge, limitAmount := range edgeLimit {
				if amt >= limitAmount {
					badEdges = append(badEdges, edge)
				}
			}
		}

		srcAlias := srcNodeInfo.Node.Alias
		if len(srcAlias) > 26 {
			srcAlias = srcAlias[:26]
		}
		dstAlias := dstNodeInfo.Node.Alias
		if len(dstAlias) > 26 {
			dstAlias = dstAlias[:26]
		}

		fmt.Printf("%d %26s -> %-26s %d %7d: ",
			srcChanId, srcAlias, dstAlias, dstChanId, amt)

		if gCfg.Verbose {
			fmt.Println()
			fmt.Printf(
				"querying possible routes, fee limit %d sat, ignoring %d edges\n",
				feeLimitFixed, len(badEdges))
		}

		// FIXME - Looks like there is a new argument to QueryRoutes:
		// https://api.lightning.community/#grpc-request-queryroutesrequest
		// Consider removing badEdges and trying use_mission_control
		// instead ...

		rsp, err := gClient.QueryRoutes(gCtx, &lnrpc.QueryRoutesRequest{
			PubKey: dstPubKey,
			Amt:    amt,
			FeeLimit: &lnrpc.FeeLimit{
				Limit: &lnrpc.FeeLimit_Fixed{
					Fixed: feeLimitFixed,
				},
			},
			SourcePubKey:   srcPubKey,
			FinalCltvDelta: int32(gCfg.Rebalance.FinalCLTVDelta),
			IgnoredEdges:   badEdges,
			IgnoredNodes:   [][]byte{ourNode},
		})

		if err != nil {
			fmt.Println("no routes found at this fee limit")
			insertLoopAttempt(NewLoopAttempt(
				time.Now().Unix(),
				srcChanId, srcPubKey,
				dstChanId, dstPubKey,
				amt, gCfg.Rebalance.FeeLimitRate,
				LoopAttemptNoRoutes,
			))
			return false
		}

		// Only get one route, only consider the first slot.
		route := rsp.Routes[0]

		// Prepend the initial hop from us through the src channel
		hop0 := &lnrpc.Hop{
			ChanId:       srcChanId,
			ChanCapacity: srcChanInfo.Capacity,
			AmtToForward: amt,
			PubKey:       srcPubKey,
			// We will set all of these when we "reprice" the route.
			// Fee:
			// Expiry:
			// AmtToForwardMsat:
			// FeeMSat:
		}
		route.Hops = append([]*lnrpc.Hop{hop0}, route.Hops...)

		// Append the final hop back to us through the dst channel
		hopN := &lnrpc.Hop{
			ChanId:       dstChanId,
			ChanCapacity: dstChanInfo.Capacity,
			AmtToForward: amt,
			PubKey:       ourPubKey,
			// We will set all of these when we "reprice" the route.
			// Fee:
			// Expiry:
			// AmtToForwardMsat:
			// FeeMSat:
		}
		route.Hops = append(route.Hops, hopN)

		repriceRoute(info, route, amt)

		if gCfg.Verbose {
			dumpRoute(info, route)
		}

		checkRoute(info, route)

		if (route.TotalFeesMsat / 1000) > feeLimitFixed {
			fmt.Println("route exceeds fee limit")
			insertLoopAttempt(NewLoopAttempt(
				time.Now().Unix(),
				srcChanId, srcPubKey,
				dstChanId, dstPubKey,
				amt, gCfg.Rebalance.FeeLimitRate,
				LoopAttemptNoRoutes,
			))
			return false
		}

		ctxt, _ :=
			context.WithTimeout(context.Background(), time.Second*60)

		if invoiceRsp == nil {
			if gCfg.Verbose {
				fmt.Println("generating invoice")
			}

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
			invoiceRsp, err = gClient.AddInvoice(ctxt, invoice)
			if err != nil {
				panic(fmt.Sprintf("unable to add invoice:", err))
			}
		}

		if gCfg.Verbose {
			fmt.Println("sending to route")
		}

		sendRsp, err := gRouter.SendToRoute(ctxt, &routerrpc.SendToRouteRequest{
			PaymentHash: invoiceRsp.RHash,
			Route:       route,
		})
		if err != nil {
			fmt.Printf("router.SendToRoute failed: %v\n", err)
			goto FailedToRoute
		}

		if sendRsp.Failure != nil {

			// errNdx is the node reporting the error.
			// errNdx == 0 means self node.
			// the hopNdx == errNdx is the failed hop.
			//
			errNdx := sendRsp.Failure.GetFailureSourceIndex()

			// If we are reporting the error let's bail on this
			// route altogether since the first hop doesn't work.
			//
			if errNdx == 0 {
				goto FailedToRoute
			}

			// This is the pubKey of the node reporting the
			// error.  We want to reject the next hop ...
			//
			pubKey := route.Hops[errNdx-1].PubKey

			// fmt.Printf("errNdx %d\n", errNdx)
			// for ndx, hop := range route.Hops {
			//     fmt.Printf("%d: %s\n", ndx, hop.PubKey)
			// }

			// Get info about the node reporting the error.
			nodeInfo0, err := gClient.GetNodeInfo(gCtx,
				&lnrpc.NodeInfoRequest{
					PubKey: pubKey,
				})
			if err != nil {
				panic(fmt.Sprintf("GetNodeInfo failed[1]:", err))
			}
			alias0 := nodeInfo0.Node.Alias

			// Get info about the target node of the failed hop.
			nodeInfo1, err := gClient.GetNodeInfo(gCtx,
				&lnrpc.NodeInfoRequest{
					PubKey: route.Hops[errNdx].PubKey,
				})
			if err != nil {
				panic(fmt.Sprintf("GetNodeInfo2 failed[1]:", err))
			}
			alias1 := nodeInfo1.Node.Alias

			fmt.Printf("%s -> %s: %s\n",
				alias0,
				alias1,
				sendRsp.Failure.Code.String())

			// Is this the last hop?  If the last hop fails this route
			// is done because we are forcing the last hop back to us ...
			//
			if int(errNdx) == len(route.Hops)-1 {
				fmt.Println("can't ignore last hop")
				goto FailedToRoute
			}

			chanId := route.Hops[errNdx].ChanId

			nextChanInfo, err :=
				gClient.GetChanInfo(gCtx, &lnrpc.ChanInfoRequest{
					ChanId: chanId,
				})
			if err != nil {
				panic(fmt.Sprintf("hop GetChanInfo failed:", err))
			}

			reverse := nextChanInfo.Node2Pub == pubKey

			if ignoreBadEdges {
				// Append this edge to the ignoredEdges and re-route.
				badEdge := &lnrpc.EdgeLocator{
					ChannelId:        chanId,
					DirectionReverse: reverse,
				}
				if gCfg.Verbose {
					fmt.Printf("ignoring %v\n", badEdge)
				}
				if edgeLimit[badEdge] == 0 || amt < edgeLimit[badEdge] {
					edgeLimit[badEdge] = amt
				}
			}
			if gCfg.Verbose {
				fmt.Println()
			}
			goto RetryQuery
		} else {
			fmt.Printf("PREIMAGE: %s\n", hex.EncodeToString(sendRsp.Preimage))
			insertLoopAttempt(NewLoopAttempt(
				time.Now().Unix(),
				srcChanId, srcPubKey,
				dstChanId, dstPubKey,
				amt, gCfg.Rebalance.FeeLimitRate,
				LoopAttemptSuccess,
			))
			return true
		}
	}

FailedToRoute:
	insertLoopAttempt(NewLoopAttempt(
		time.Now().Unix(),
		srcChanId, srcPubKey,
		dstChanId, dstPubKey,
		amt, gCfg.Rebalance.FeeLimitRate,
		LoopAttemptFailure,
	))
	return false
}
