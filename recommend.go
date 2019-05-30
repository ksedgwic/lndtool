// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"time"
	
    "github.com/lightningnetwork/lnd/lnrpc"
)

type PotentialLoop struct {
	SrcChan uint64
	SrcNode string
	DstChan uint64
	DstNode string
	Amount int64
}

func NewPotentialLoop(
	srcChan uint64,
	srcNode string,
	dstChan uint64,
	dstNode string,
	amount int64,
) *PotentialLoop {
	return &PotentialLoop{
		SrcChan: srcChan,
		SrcNode: srcNode,
		DstChan: dstChan,
		DstNode: dstNode,
		Amount: amount,
	}
}

var minImbalance = int64(1000)
var feeLimitRate = float64(0.0005)
var amountLimit = int64(10000)
var recentSecs = int64(2 * 60 * 60)

func recommend(client lnrpc.LightningClient, ctx context.Context, db *sql.DB,
	args []string) {

	rsp, err := client.ListChannels(ctx, &lnrpc.ListChannelsRequest{
		ActiveOnly: true,
		InactiveOnly: false,
		PublicOnly: true,
		PrivateOnly: false,
	})
    if err != nil {
		panic(fmt.Sprint("ListChannels failed:", err))
    }

	// Consider all combinations of channels
	loops := []*PotentialLoop{}
	for srcNdx, srcChan := range rsp.Channels {
		for dstNdx, dstChan := range rsp.Channels {

			// Same channel can't be both src and dst:
			if srcNdx == dstNdx {
				continue
			}

			// Make sure the connected nodes are unique:
			if srcChan.RemotePubkey == dstChan.RemotePubkey {
				continue
			}

			// Make sure the source imbalance is positive:
			srcImbalance :=
				srcChan.LocalBalance -
				((srcChan.LocalBalance + srcChan.RemoteBalance) / 2)
			if srcImbalance < minImbalance {
				continue
			}
			
			// Make sure the destination imbalance is negative:
			dstImbalance :=
				dstChan.LocalBalance -
				((dstChan.LocalBalance + dstChan.RemoteBalance) / 2)
			if dstImbalance > -minImbalance {
				continue
			}

			// What is the target amount to be moved?
			amount := int64(0)
			if srcImbalance < -dstImbalance {
				amount = srcImbalance
			} else {
				amount = -dstImbalance
			}

			loops = append(loops, NewPotentialLoop(
				srcChan.ChanId, srcChan.RemotePubkey,
				dstChan.ChanId, dstChan.RemotePubkey,
				amount,
			))
		}
	}

	sort.SliceStable(loops, func(ii, jj int) bool {
		// Amount descending
		return loops[ii].Amount > loops[jj].Amount
	})

	for _, loop := range loops {
		// Has this loop already failed recently?
		// Don't include history prior to the horizon.
		// Don't include higher amounts than this one.
		// Don't include lessor feeLimitRates than this one.

		// Limit the rebalance amount
		amount := loop.Amount
		if amount > amountLimit {
			amount = amountLimit
		}
			
		// Consider recent history
		tstamp := time.Now().Unix() - recentSecs
		if !recentlyFailed(db, loop.SrcChan, loop.DstChan, tstamp, amount, feeLimitRate) {

			fmt.Printf("./lndtool rebalance %d %d %d %f\n",
				amount, loop.SrcChan, loop.DstChan, feeLimitRate)
			os.Exit(0)
		}
	}

	fmt.Println("no loops recommended")
	os.Exit(1)
}
