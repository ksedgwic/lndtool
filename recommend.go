// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
	
    "github.com/lightningnetwork/lnd/lnrpc"
    "github.com/lightningnetwork/lnd/lnrpc/routerrpc"
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

func recommend(cfg *config, client lnrpc.LightningClient, router routerrpc.RouterClient, ctx context.Context, db *sql.DB, args []string) bool {

	var blacklist = map[string]bool{}
	for _, node := range cfg.Recommend.PeerNodeBlacklist {
		blacklist[node] = true
	}
	
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
		
		// Is this node blacklisted?
		if blacklist[srcChan.RemotePubkey] {
			continue
		}
		
		for dstNdx, dstChan := range rsp.Channels {
			
			// Is this node blacklisted?
			if blacklist[dstChan.RemotePubkey] {
				continue
			}
			
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
			if srcImbalance < cfg.Recommend.MinImbalance {
				continue
			}
			
			// Make sure the destination imbalance is negative:
			dstImbalance :=
				dstChan.LocalBalance -
				((dstChan.LocalBalance + dstChan.RemoteBalance) / 2)
			if dstImbalance > -cfg.Recommend.MinImbalance {
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
		// Limit the rebalance amount
		amount := loop.Amount
		if amount > cfg.Recommend.TransferAmount {
			amount = cfg.Recommend.TransferAmount
		}
			
		// Consider recent history
		tstamp := time.Now().Unix() - int64(cfg.Recommend.RetryInhibit.Seconds())
		if !recentlyFailed(db, loop.SrcChan, loop.DstChan, tstamp, amount, cfg.Rebalance.FeeLimitRate) {

			fmt.Printf("./lndtool rebalance %d %d %d\n",
				amount, loop.SrcChan, loop.DstChan)
			doRebalance(cfg, client, router, ctx, db, amount, loop.SrcChan, loop.DstChan)
			return true
		}
	}

	fmt.Println("no loops recommended")
	return false
}
